package main

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path"
	"strings"
)

const partiqlURLTemplate = "https://github.com/partiql/partiql-lang-kotlin/releases/download/v%s-alpha/partiql-cli-%s.tgz"

var includedJarPrefixes = []string{
	"cli",
	"jopt-simple",
	"ion-java",
	"lang",
	"kotlin-stdlib",
	"partiql-ir-generator-runtime",
	"ion-element",
}

// isIncludedJar returns true if filePath is one of the jars included in includedJarPrefixes
func isIncludedJar(filePath string) bool {
	// the file name must start with one of the prefixes, then "-" + version
	filePart := path.Base(filePath)
	for _, prefix := range includedJarPrefixes {
		prefixDash := prefix + "-"
		if strings.HasPrefix(filePart, prefixDash) {
			nextByte := filePart[len(prefixDash)]
			if '0' <= nextByte && nextByte <= '9' {
				return true
			}
		}
	}
	return false
}

func httpUntarToJar(url string, jarW *jarWriter) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if !(200 <= resp.StatusCode && resp.StatusCode < 300) {
		return fmt.Errorf("unexpected status=%s", resp.Status)
	}

	unzipped, err := gzip.NewReader(resp.Body)
	if err != nil {
		return err
	}
	defer unzipped.Close()

	reader := tar.NewReader(unzipped)
	for {
		header, err := reader.Next()
		if err != nil {
			if err == io.EOF {
				break
			}
			return err
		}

		if !header.FileInfo().Mode().IsRegular() {
			continue
		}
		if !strings.HasSuffix(header.Name, ".jar") {
			continue
		}

		if isIncludedJar(header.Name) {
			fmt.Printf("jar name=%s is being included ...\n", header.Name)
			err = combineJar(jarW, reader)
			if err != nil {
				return err
			}
		} else {
			fmt.Printf("jar name=%s is not included\n", header.Name)
		}
	}

	err = unzipped.Close()
	if err != nil {
		return err
	}
	return resp.Body.Close()
}

type jarWriter struct {
	zw             *zip.Writer
	alreadyWritten map[string]struct{}
}

func (w *jarWriter) copy(path string, data io.Reader) error {
	// do nothing if we wrote a previous version
	if _, exists := w.alreadyWritten[path]; exists {
		fmt.Printf("  warning: skipping duplicate file %s\n", path)
		return nil
	}
	w.alreadyWritten[path] = struct{}{}

	out, err := w.zw.Create(path)
	if err != nil {
		return err
	}
	_, err = io.Copy(out, data)
	return err
}

func combineJar(jarW *jarWriter, jarReader io.Reader) error {
	// read the jar bytes from jarReader; then open it as a zip file
	jarBytes, err := ioutil.ReadAll(jarReader)
	if err != nil {
		return err
	}
	reader, err := zip.NewReader(bytes.NewReader(jarBytes), int64(len(jarBytes)))
	if err != nil {
		return err
	}

	// copy every file in the jar to the jarWriter
	for _, f := range reader.File {
		if !f.Mode().IsRegular() {
			continue
		}

		// fmt.Printf("  copying %s ...\n", f.Name)
		r, err := f.Open()
		if err != nil {
			return err
		}
		err = jarW.copy(f.Name, r)
		err2 := r.Close()
		if err != nil {
			return err
		}
		if err2 != nil {
			return err2
		}
	}
	return nil
}

func main() {
	version := flag.String("version", "0.2.4", "PartiQL version to download")
	outputPath := flag.String("outputPath", "", "Path to write the combined jar")
	flag.Parse()
	if *outputPath == "" {
		fmt.Fprintln(os.Stderr, "Usage: combinejars --outputPath=(path to output JAR)")
		os.Exit(1)
	}

	f, err := os.OpenFile(*outputPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0600)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	zw := zip.NewWriter(f)
	defer zw.Close()

	jarW := &jarWriter{zw, make(map[string]struct{})}

	partiqlURL := fmt.Sprintf(partiqlURLTemplate, *version, *version)
	log.Printf("downloading PartiQL version=%s from %s ...", *version, partiqlURL)
	err = httpUntarToJar(partiqlURL, jarW)
	if err != nil {
		panic(err)
	}
	err = zw.Close()
	if err != nil {
		panic(err)
	}
	err = f.Close()
	if err != nil {
		panic(err)
	}
}
