const modules = require('module')
    .builtinModules.filter((m) => !m.startsWith('_'))
    .sort();

console.log(modules.join('\n'));
