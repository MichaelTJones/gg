
# gg is grep for Go-language source code

It restricts the search to
designated Go token classes, such as identifiers, package names, numbers, comments, keywords, and
the rest. Because gg understands what it is searching for, it can make smart matches. For
example:

* Searching for numbers by _value_ rather than regular expression: find 255
expressed as 0b1111_1111, 0377, 255, or 0xff with "gg v 255 *.go". Note: this is a value
("v") search
as opposed to a number ("n") search. Values must be valid  Go integer or floating point
literals (22, 0xface, 6.02214076e23, 0o644).

* Searching for "if" in Go keywords, but not in comments or strings, is "gg k if ." for _keywords_ matching "if" in all the ".go" files in the current directory.

* Searching a file hierarchy recursively for _comments_ containing "case" (ignoring
  switch statements), is "gg -r c case ."

* gg has a grep mode, "-g" which omits the Go grammar tokenization. This mode is generally
twice as fast as standard gg, and even faster compared to classic grep. Related is "-go=false" to allow scanning of non-Go files.

## Documentation

gg does much more. Please see the [man
page](https://github.com/MichaelTJones/gg/blob/master/gg.pdf) for details.

## Installation

```go
go get github.com/MichaelTJones/gg
cd gg
go install
```
