#!/bin/bash

source VERSION

set -e

function ask_yes_or_no() {
    read -p "$1 (y/n): "
    case $(echo $REPLY | tr '[A-Z]' '[a-z]') in
        y|yes) echo "yes" ;;
        *)     echo "no" ;;
    esac
}

function bump_major(){
    NEW_VERSION=$(($MAJOR+1)).0.0
    echo "Version is: $NEW_VERSION"
    if [[ "yes" == $(ask_yes_or_no "is this what you want?") ]]; then
        sed -i "s/MAJOR=.*/MAJOR=$(($MAJOR + 1))/" VERSION
        sed -i "s/MINOR=.*/MINOR=0/" VERSION
        sed -i "s/PATCH=.*/PATCH=0/" VERSION
    fi
}

function bump_minor(){
    NEW_VERSION=$MAJOR.$(($MINOR+1)).0
    echo "Version is: $NEW_VERSION"
    if [[ "yes" == $(ask_yes_or_no "is this what you want?") ]]; then
        sed -i "s/MINOR=.*/MINOR=$(($MINOR + 1))/" VERSION
        sed -i "s/PATCH=.*/PATCH=0/" VERSION
    fi
}

function bump_patch(){
    NEW_VERSION=$MAJOR.$MINOR.$(($PATCH+1))
    echo "Version is: $NEW_VERSION"
    if [[ "yes" == $(ask_yes_or_no "is this what you want?") ]]; then
        sed -i "s/PATCH=.*/PATCH=$(($PATCH + 1))/" VERSION
    fi
}

function generate_changelog(){
    echo "## $(date +%c) Version: $NEW_VERSION" >> CHANGELOG.md
}

function usage(){
    printf "Usage: $0 -M [major] -m [minor] -p [patch] -h [print this message]\n"
}

if [ -z "$1" ]; then
    usage
    exit -1
fi

while getopts "hmMp" opt; do
    case "$opt" in
        h) usage
            ;;
        m) bump_minor
            generate_changelog
            exit 0
            ;;
        M) bump_major
            generate_changelog
            exit 0
            ;;
        p) bump_patch
            generate_changelog
            exit 0
            ;;
    esac
done
