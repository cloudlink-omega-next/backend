const fs = require('fs');
const path = require('path');

const filePath = path.join(__dirname, 'i18n.js');
let content = fs.readFileSync(filePath, 'utf8');

// Fix unescaped single quotes in specific terms
content = content.replace(/explicitly/g, "explicitly");
content = content.replace(/offending/g, "offending");
content = content.replace(/deem/g, "deem");

fs.writeFileSync(filePath, content, 'utf8');
console.log('Fixed i18n.js');
