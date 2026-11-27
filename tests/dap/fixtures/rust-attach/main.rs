use std::{fs, thread, time::Duration};

extern "C" {
    fn getppid() -> i32;
    fn prctl(option: i32, arg: usize, zero1: usize, zero2: usize, zero3: usize) -> i32;
}

fn main() {
    unsafe { assert_eq!(prctl(0x59616d61, getppid() as usize, 0, 0, 0), 0); }
    fs::write("pid", std::process::id().to_string()).unwrap();
    println!("TARGET_READY");
    for i in 0i32.. {
        let value = 42;
        let pulse = i + value;
        fs::write("heartbeat", pulse.to_string()).unwrap();
        thread::sleep(Duration::from_millis(25));
    }
}
