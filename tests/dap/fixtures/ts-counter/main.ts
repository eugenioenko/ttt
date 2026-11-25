function increment(value: number) {
    return value + 1;
}
function run(initial: number) {
    let count = initial;
    count++;
    count = increment(count);
    console.log("count =", count);
}
run(41);
