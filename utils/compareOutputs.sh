#!/bin/zsh

rm -rf ./gg
go build -o gg main.go scan.go
./gg aS grep . > ./new
gg aS grep . > ./old

CHANGES=$(diff ./new ./old | wc -l)

if [ $CHANGES -eq 0 ]; then
    echo "test1: everything is still working"
    rm -rf ./new ./old
else
    echo "test1: outputs don't match"
    diff ./new ./old
    return
fi

./gg a grep . > ./new
gg a grep . > ./old

CHANGES=$(diff ./new ./old | wc -l)

if [ $CHANGES -eq 0 ]; then
    echo "test2: everything is still working"
    rm -rf ./new ./old
else
    echo "test2: outputs don't match"
    diff ./new ./old
    return
fi

./gg aV grep . > ./new
gg aV grep . > ./old

CHANGES=$(diff ./new ./old | wc -l)

if [ $CHANGES -eq 0 ]; then
    echo "test3: everything is still working"
    rm -rf ./new ./old
else
    echo "test3: outputs don't match"
    diff ./new ./old
    return
fi
