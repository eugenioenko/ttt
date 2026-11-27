#include <stdio.h>
#include <unistd.h>
#include <sys/prctl.h>

int main(void) {
    if (prctl(PR_SET_PTRACER, getppid(), 0, 0, 0)) return 1;
    FILE *pidfile = fopen("pid", "w");
    fprintf(pidfile, "%d", getpid());
    fclose(pidfile);
    puts("TARGET_READY");
    fflush(stdout);
    for (int i = 0; ; i++) {
        int value = 42;
        FILE *pulse = fopen("heartbeat", "w");
        fprintf(pulse, "%d", i + value);
        fclose(pulse);
        usleep(25000);
    }
}
