
# gg: Go Grep

gg is a grep tool for Go-language source code. It restricts the search to
designated Go token classes, such as identifiers, package names, numbers, comments, keywords, and
the like. Because gg understands what it is searching for, it can make smart matches. For
example:

* Searching for _numbers_ that match a value, say 255, no matter if expressed as 0b1111_1111, 0377, 255, or 0xff is easy with "gg -n 255 *.go"

* Searching for "if" in code, but not in comments or strings, is "gg -k if ." for _keywords_ matching "if" in all the ".go" files in the current directory.

* Searching a file hierarchy _recursively_ for _comments_ containing "case" (but not
  switch statements), is "gg -r -c case ."
