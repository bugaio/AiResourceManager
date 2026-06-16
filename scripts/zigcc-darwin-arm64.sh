#!/bin/bash
# macOS ARM64 - 直接用系统 clang（原生架构）
exec clang -arch arm64 "$@"
