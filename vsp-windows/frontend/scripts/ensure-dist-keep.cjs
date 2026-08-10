const fs = require('fs')
const path = require('path')

const distDir = path.join(__dirname, '..', 'dist')
fs.mkdirSync(distDir, { recursive: true })
fs.closeSync(fs.openSync(path.join(distDir, '.gitkeep'), 'a'))
