fn increment(value: i32) -> i32 {
    value + 1
}
fn main() {
    let mut count: i32 = 41;
    count += 1;
    count = increment(count);
    println!("count = {count}");
}
