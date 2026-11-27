declare function require(name: string): { writeFileSync(path: string, data: string): void };
declare const process: { debugPort: number; pid: number };
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
