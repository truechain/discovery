#!/bin/sh

set -e

if [ ! -f "build/env.sh" ]; then
    echo "$0 must be run from the root of the repository."
    exit 2
fi

# Create fake Go workspace if it doesn't exist yet.
workspace="$PWD/build/_workspace"
root="$PWD"
truedir="$workspace/src/github.com/truechain"
if [ ! -L "$truedir/discovery" ]; then
    mkdir -p "$truedir"
    cd "$truedir"
    ln -s ../../../../../. discovery
    cd "$root"
fi

# Set up the environment to use the workspace.
GOPATH="$workspace"
export GOPATH

# Run the command inside the workspace.
cd "$truedir/discovery"
PWD="$truedir/discovery"

# Launch the arguments with the configured environment.
exec "$@"
