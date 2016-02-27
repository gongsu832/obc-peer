#!/bin/bash

if [ -z $GOPATH ]; then
  echo GOPATH must be set
  exit
fi

if [ "$1" != "restore" ]; then
  GOLANGLIST=`ls -d vendor/**|grep -v github|grep -v json`
  if [ -z "$GOLANGLIST" ]; then
    echo "vendor subdirectories already relocated to $GOPATH/src"
    exit
  fi

  GITHUBLIST=`ls -d vendor/github.com/**`

  echo -n "Relocate vendor subdirectories to $GOPATH/src ... "
  mv $GOLANGLIST $GOPATH/src/
  mv $GITHUBLIST $GOPATH/src/github.com/
  rmdir vendor/github.com
else
  GOLANGLIST=`ls -d $GOPATH/src/**|grep -v github`
  if [ -z "$GOLANGLIST" ]; then
    echo "vendor subdirectories already restored from $GOPATH/src"
    exit
  fi

  GITHUBLIST=`ls -d $GOPATH/src/github.com/**|grep -v openblockchain`

  echo -n "Restore vendor subdirectories from $GOPATH/src ... "
  mkdir vendor/github.com
  mv $GOLANGLIST vendor/
  mv $GITHUBLIST vendor/github.com/
fi

echo "Done"
