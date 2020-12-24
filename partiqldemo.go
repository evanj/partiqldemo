package main

import (
	"flag"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
)

const portEnvVar = "PORT"
const defaultPort = "8080"
const executePath = "/execute"
const envFormID = "env"
const queryFormID = "query"
const mainClass = "org.partiql.cli.Main"

func writeTemp(data string) (*os.File, error) {
	tempFile, err := ioutil.TempFile("", "")
	if err != nil {
		return nil, err
	}
	_, err = tempFile.Write([]byte(data))
	if err != nil {
		os.Remove(tempFile.Name())
		return nil, err
	}
	err = tempFile.Close()
	if err != nil {
		os.Remove(tempFile.Name())
		return nil, err
	}

	return tempFile, nil
}

type queryExecError struct {
	output []byte
	err    error
}

func (q *queryExecError) Error() string {
	return fmt.Sprintf("query exec: %s; original err %s", string(q.output), q.err.Error())
}

func (q *queryExecError) Unwrap() error {
	return q.err
}

// executeOriginalCLI executes the upstream org.partiql.cli.Main class. Use this function
// to use the unmodified upstream distribution. As of the most recent release, its output
// format is not quite as nice.
func executeOriginalCLI(classpath string, query string, environment string) (string, error) {
	// write the environment data to a temporary file
	tempFile, err := writeTemp(environment)
	if err != nil {
		return "", err
	}
	defer os.Remove(tempFile.Name())

	// execute the original CLI
	var args []string
	if classpath != "" {
		args = append(args, "-classpath", classpath)
	}
	args = append(args, mainClass, "--environment", tempFile.Name(), "--output-format", "PARTIQL",
		"--query", query)
	cmd := exec.Command("java", args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", &queryExecError{out, err}
	}

	return string(out), nil
}

func executeNewCLI(jar string, query string, environment string) (string, error) {
	// write the environment data to a temporary file
	tempFile, err := writeTemp(environment)
	if err != nil {
		return "", err
	}
	defer os.Remove(tempFile.Name())

	// execute the new CLI
	cmd := exec.Command("java", "-jar", jar, tempFile.Name())

	// write the query on stdin in a separate goroutine
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return "", err
	}
	writeErr := make(chan error, 1)
	go func() {
		_, err := stdin.Write([]byte(query))
		err2 := stdin.Close()
		if err != nil {
			writeErr <- err
		}
		writeErr <- err2
	}()

	// get the result and any execution error
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", &queryExecError{out, err}
	}

	// check that we wrote to stdin okay
	err = <-writeErr
	if err != nil {
		return "", err
	}

	return string(out), nil
}

func (s *server) handleRoot(w http.ResponseWriter, r *http.Request) {
	log.Printf("handleRoot %s %s", r.Method, r.URL.String())
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "invalid method", http.StatusMethodNotAllowed)
		return
	}

	err := s.executeAndRender(w, tutorialQuery, tutorialData)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (s *server) handleExecute(w http.ResponseWriter, r *http.Request) {
	log.Printf("handleExecute %s %s", r.Method, r.URL.String())
	if r.Method != http.MethodPost {
		http.Error(w, "invalid method", http.StatusMethodNotAllowed)
		return
	}
	err := s.handleExecuteErr(w, r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

type server struct {
	classpath string
	jarPath   string
}

func (s *server) handleExecuteErr(w http.ResponseWriter, r *http.Request) error {
	err := r.ParseForm()
	if err != nil {
		return err
	}

	queryData := r.FormValue(queryFormID)
	envData := r.FormValue(envFormID)
	if queryData == "" || envData == "" {
		return fmt.Errorf("query and environment must not be empty")
	}

	return s.executeAndRender(w, queryData, envData)
}

func (s *server) executeAndRender(w http.ResponseWriter, query string, envData string) error {
	var result string
	var err error
	if s.jarPath != "" {
		result, err = executeNewCLI(s.jarPath, query, envData)
	} else {
		result, err = executeOriginalCLI(s.classpath, query, envData)
	}
	if err != nil {
		return err
	}

	values := &rootTemplateValues{query, envData, result}
	return rootTemplate.Execute(w, values)
}

func (s *server) makeHandlers() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleRoot)
	mux.HandleFunc(executePath, s.handleExecute)
	return mux
}

func (s *server) serveHTTP(addr string) error {
	mux := s.makeHandlers()
	log.Printf("listening on http://%s ...", addr)
	return http.ListenAndServe(addr, mux)
}

func main() {
	classpath := flag.String("classpath", "", "-classpath argument for the original CLI")
	jarPath := flag.String("jar", "", "path to jar for the new CLI")
	addr := flag.String("addr", "", "If set, address for HTTP requests.")
	flag.Parse()

	if *addr == "" && os.Getenv(portEnvVar) != "" {
		*addr = ":" + os.Getenv(portEnvVar)
	} else if *addr == "" {
		*addr = ":" + defaultPort
	}

	s := &server{*classpath, *jarPath}
	err := s.serveHTTP(*addr)
	if err != nil {
		panic(err)
	}
}

type rootTemplateValues struct {
	Query   string
	Data    string
	Results string
}

var rootTemplate = template.Must(template.New("root").Parse(`<!doctype html>
<html>
<head><title>PartiQL Explorer</title></head>
<body>
<h1>PartiQL Explorer</h1>
<p>Execute PartiQL queries using the CLI tool.</p>

<form method="post" action="` + executePath + `">
<h2>Query</h2>
<textarea name="` + queryFormID + `" rows="10" cols="120" wrap="off" autofocus>{{.Query}}</textarea>
<p><input type="submit" value="Execute"></p>

<h2>Results</h2>
<pre>{{.Results}}</pre>

<h2>Data</h2>
<textarea name="` + envFormID + `" rows="10" cols="120" wrap="off" autofocus>{{.Data}}</textarea>
</form>
</body>
</html>
`))

const tutorialQuery = `-- query from the PartiQL tutorial
SELECT e.name AS employeeName, 
       e.project.name AS projectName
FROM hr.employeesWithTuples e
WHERE e.project.org = 'AWS'
`

const tutorialData = `-- contents of tutorial-all-data.env from PartiQL CLI
{ 
    'hr': { 
        'employees': <<
            { 'id': 3, 'name': 'Bob Smith',   'title': null }, -- a tuple is denoted by { ... } in the PartiQL data model
            { 'id': 4, 'name': 'Susan Smith', 'title': 'Dev Mgr' },
            { 'id': 6, 'name': 'Jane Smith',  'title': 'Software Eng 2'}
        >>, 
      'employeesNest': <<
         { 
          'id': 3, 
          'name': 'Bob Smith', 
          'title': null, 
          'projects': [ { 'name': 'AWS Redshift Spectrum querying' },
                        { 'name': 'AWS Redshift security' },
                        { 'name': 'AWS Aurora security' }
                      ]
          },
          { 
              'id': 4, 
              'name': 'Susan Smith', 
              'title': 'Dev Mgr', 
              'projects': [] 
          },
          { 
              'id': 6, 
              'name': 'Jane Smith', 
              'title': 'Software Eng 2', 
              'projects': [ { 'name': 'AWS Redshift security' } ] 
          }
      >>,
      'employeesWithTuples': << 
            { 
                'id': 3, 
                'name': 'Bob Smith', 
                'title': null, 
                'project': { 
                    'name': 'AWS Redshift Spectrum querying', 
                    'org': 'AWS' 
                }
            },
            {
                'id': 6, 
                'name': 'Jane Smith', 
                'title': 'Software Eng 2', 
                'project': { 
                    'name': 'AWS Redshift security', 
                    'org': 'AWS' 
                }
            }
         >>, 
         'employeesNestScalars': <<
            { 
                'id': 3, 
                'name': 'Bob Smith', 
                'title': null, 
                'projects': [ 
                    'AWS Redshift Spectrum querying',
                    'AWS Redshift security',
                    'AWS Aurora security'
                ]
            },
            { 
                'id': 4, 
                'name': 'Susan Smith', 
                'title': 'Dev Mgr', 
                'projects': []
            },
            { 
                'id': 6, 
                'name': 'Jane Smith', 
                'title': 'Software Eng 2', 
                'projects': [ 'AWS Redshift security' ]
            }
        >>,
        'employeesWithMissing': <<
            { 'id': 3, 'name': 'Bob Smith' }, -- no title in this tuple
            { 'id': 4, 'name': 'Susan Smith', 'title': 'Dev Mgr' },
            { 'id': 6, 'name': 'Jane Smith', 'title': 'Software Eng 2'}
        >>,
        'employeesMixed2': <<
            { 
                'id': 3, 
                'name': 'Bob Smith', 
                'title': null, 
                'projects': [ 
                    { 'name': 'AWS Redshift Spectrum querying' },
                    'AWS Redshift security',
                    { 'name': 'AWS Aurora security' }
                ]
            },
            { 
                'id': 4, 
                'name': 'Susan Smith', 
                'title': 'Dev Mgr', 
                'projects': []
            },
            { 
                'id': 6, 
                'name': 'Jane Smith', 
                'projects': [ 'AWS Redshift security'] 
            }
        >>

    },
    'matrices': <<
        { 
            'id': 3, 
            'matrix': [ 
                [2, 4, 6],
                [1, 3, 5, 7],
                [9, 0]
            ]
        },
        { 
            'id': 4, 
            'matrix': [ 
                [5, 8],
                [ ]
            ]
            
        }
    >>, 
    'closingPrices': <<
        { 'date': '4/1/2019', 'amzn': 1900, 'goog': 1120, 'fb': 180 },
        { 'date': '4/2/2019', 'amzn': 1902, 'goog': 1119, 'fb': 183 }
    >>,
    'todaysStockPrices': <<
        { 'symbol': 'amzn', 'price': 1900},
        { 'symbol': 'goog', 'price': 1120},
        { 'symbol': 'fb', 'price': 180 }
    >>, 
    'stockPrices':<<
        { 'date': '4/1/2019', 'symbol': 'amzn', 'price': 1900},
        { 'date': '4/1/2019', 'symbol': 'goog', 'price': 1120},
        { 'date': '4/1/2019', 'symbol': 'fb',   'price': 180 },
        { 'date': '4/2/2019', 'symbol': 'amzn', 'price': 1902},
        { 'date': '4/2/2019', 'symbol': 'goog', 'price': 1119},
        { 'date': '4/2/2019', 'symbol': 'fb',   'price': 183 }
    >>
} 
`