"use strict";
function increment(value) {
    return value + 1;
}
function run(initial) {
    let count = initial;
    count++;
    count = increment(count);
    console.log("count =", count);
}
run(41);
//# sourceMappingURL=main.js.map