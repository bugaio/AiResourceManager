#!/bin/bash
exec zig cc -target aarch64-windows-gnu "$@"
