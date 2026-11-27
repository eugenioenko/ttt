"use strict";
const fs = require("node:fs");
fs.writeFileSync("port", String(process.debugPort));
fs.writeFileSync("pid", String(process.pid));
console.log("TARGET_READY");
let i = 0;
setInterval(() => {
    const value = 40 + 2;
    fs.writeFileSync("heartbeat", String(i + value));
    i++;
}, 25);
//# sourceMappingURL=app.js.map