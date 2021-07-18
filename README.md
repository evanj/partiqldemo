# PartiQL Demo

This is a Go web server that wraps the [PartiQL](https://partiql.org/) command line tools in a web
interface, to make it easier to use. [Try it with a version running on Google Cloud Run](https://partiqldemo.evanjones.ca/).


## Usage

1. Build it with `make`
2. Run it with `go run . --jar=build/partiqldemo.jar`
3. Visit http://localhost:8080/


## Java input

The command line tool reads the "environment" a file, then reads the query from standard input. It then executes the query and prints the output on standard output.

This unfortunately requires starting the JVM then tearing it down. To make the web interface work with lower latency, it also supports a server mode. It reads input from standard input using the following protocol:

* 4 bytes little endian: length of query
* 4 bytes little endian: length of environment
* query bytes (UTF-8)
* environment bytes (UTF-8)

After reading the input, it writes to standard out

* 4 bytes little endian: length of output
* output bytes (UTF-8)
