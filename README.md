
# survey

Survey Go-language source code and report usage statistics. Builds reports like this:

* [Go 1.13 source code](https://gist.github.com/MichaelTJones/ca0fd339401ebbe79b9cbb5044afcfe2)

* [Russ Cox's Go Corpus](https://gist.github.com/MichaelTJones/609589e05017da4be52bc2810e9df4e8)

This is a concurrent Go program. Surveying the [Go Corpus](https://github.com/rsc/corpus)
on my personal computer (after splitting per the comments in split.go) parses those 752
MiB of Go source at 450 MiB/sec. This is a testament to Go's efficiency.
