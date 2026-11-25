"use strict";
function report(arg) {
    const local = arg + 1;
    const payload = { answer: local };
    console.log("TS_RESULT", local);
    console.log("TS_DONE", local + 1);
}
report(41);
console.log("AFTER_REPORT");
//# sourceMappingURL=app.js.map