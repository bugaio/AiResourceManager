#!/bin/bash
# macOS AMD64 - 用系统 clang 交叉编译（Apple clang 支持多架构）
exec clang -arch x86_64 "$@"
