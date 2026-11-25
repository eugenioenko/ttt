#include <stdio.h>
static int increment(int value) {
    return value + 1;
}
int main(void) {
    int count = 41;
    count++;
    count = increment(count);
    printf("count = %d\n", count);
    return 0;
}
