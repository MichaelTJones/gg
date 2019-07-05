
# gg is grep for Go-language source code

It restricts the search to
designated Go token classes, such as identifiers, package names, numbers, comments, keywords, and
the rest. Because gg understands what it is searching for, it can make smart matches. For
example:

* Searching for _numbers_ by value,  255 for example, no matter if expressed as
  0b1111_1111, 0377, 255, or 0xff is easy with "gg v 255 *.go". Note: this is a value ("v")
  search as opposed to the number ("n") search. Numbers support literals like 255 and
  regular expression patterns
  like "2\[0-9\]\." but values must be valid  Go integer or floating point
  literals (22, 0xface, 6.02214076e23).

* Searching for "if" in Go keywords, but not in comments or strings, is "gg k if ." for _keywords_ matching "if" in all the ".go" files in the current directory.

* Searching a file hierarchy recursively for _comments_ containing "case" (ignoring
  switch statements), is "gg -r c case ."

gg does much more. Details are in the manpage: gg.pdf

## Installation

```go
go get github.com/MichaelTJones/gg
cd gg
go install
```
