#!/bin/bash
# 构建前端静态资源（GoReleaser before hook 调用）
set -e
cd web
npm install
npm run build
