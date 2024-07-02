#!/bin/bash

installdir=/opt/dcnetio
if [ $(id -u) -ne 0 ]; then
    echo "Please run with sudo!"
    exit 1
fi
if [ -d $installdir ] && [ -d $installdir/bin ]; then
    echo "Uninstalling dc..."
    echo "Stopping dc..."
    $installdir/bin/dc stop all
    echo "Removing dc..."
    #remove  container name with dcstorage
    existFlag=$(docker ps -a | grep dcstorage | awk '{print $1}')
    if [ -n "$existFlag" ]; then
        docker ps -a | grep dcstorage | awk '{print $1}' | xargs docker rm -f
    fi
    #remove  container name with dcupgrade
    existFlag=$(docker ps -a | grep dcupgrade | awk '{print $1}')
    if [ -n "$existFlag" ]; then
         docker ps -a | grep dcupgrade | awk '{print $1}' | xargs docker rm -f
    fi
    #remove  container name with dcchain
    existFlag=$(docker ps -a | grep dcchain | awk '{print $1}')
    if [ -n "$existFlag" ]; then
         docker ps -a | grep dcchain | awk '{print $1}' | xargs docker rm -f
    fi
    #remove  container name with dcpccs
    existFlag=$(docker ps -a | grep dcpccs | awk '{print $1}')
    if [ -n "$existFlag" ]; then
        docker ps -a | grep dcpccs | awk '{print $1}' | xargs docker rm -f
    fi
    #remove  image tag with ghcr.io/dcnetio/dcstorage
    existFlag=$(docker images | grep dcnetio/dcstorage | awk '{print $3}')
    if [ -n "$existFlag" ]; then
        docker images | grep dcnetio/dcstorage | awk '{print $3}' | xargs docker rmi -f
    fi
    #remove  image tag with ghcr.io/dcnetio/dcupgrade
    existFlag=$(docker images | grep dcnetio/dcupgrade | awk '{print $3}')
    if [ -n "$existFlag" ]; then
       docker images | grep dcnetio/dcupgrade | awk '{print $3}' | xargs docker rmi -f
    fi
    #remove  image tag with ghcr.io/dcnetio/dcchain
    existFlag=$(docker images | grep dcnetio/dcchain | awk '{print $3}')
    if [ -n "$existFlag" ]; then
       docker images | grep dcnetio/dcchain | awk '{print $3}' | xargs docker rmi -f
    fi
    #remove  image tag with ghcr.io/dcnetio/pccs
    existFlag=$(docker images | grep dcnetio/pccs | awk '{print $3}')
    if [ -n "$existFlag" ]; then
       docker images | grep dcnetio/pccs | awk '{print $3}' | xargs docker rmi -f
    fi
    #remove volume name with dcstorage
    existFlag=$(docker volume ls  | grep dcstorage | awk '{print $2}')
    if [ -n "$existFlag" ]; then
        docker volume ls | grep dcstorage | awk '{print $2}' | xargs docker volume rm -f
    fi
    #remove volume name with dcpccs
    existFlag=$(docker volume ls  | grep dcpccs | awk '{print $2}')
    if [ -n "$existFlag" ]; then
        docker volume ls | grep dcpccs | awk '{print $2}' | xargs docker volume rm -f
    fi
    rm -rf $installdir
    rm -rf /usr/bin/dc
    rm -rf /etc/bash_completion.d/dc
    echo "Uninstalling dc done."
 else
    echo "dc is not installed!"
fi

