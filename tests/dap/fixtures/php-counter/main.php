<?php
function increment($value) {
    return $value + 1;
}
function run($initial) {
    $count = $initial;
    $count++;
    $count = increment($count);
    echo "count = $count\n";
}
if (($argv[1] ?? '') === 'service') {
    file_put_contents('pid', (string)getmypid());
    echo "TARGET_READY\n";
    for ($i = 0; ; $i++) {
        $value = 42;
        file_put_contents('heartbeat', (string)($i + $value));
        usleep(25000);
    }
}
run((int)($argv[1] ?? 41));
