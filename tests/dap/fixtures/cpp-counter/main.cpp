#include <cstdio>
#include <sys/prctl.h>
#include <unistd.h>

struct Counter {
    int total;
    int add(int value) {
        total += value;
        return total;
    }
};

int main(int argc, char **) {
    if (argc > 1) {
        if (prctl(PR_SET_PTRACER, getppid(), 0, 0, 0)) return 1;
        FILE *pid = std::fopen("pid", "w");
        std::fprintf(pid, "%d", getpid());
        std::fclose(pid);
        std::puts("TARGET_READY");
        std::fflush(stdout);
        for (int i = 0; ; i++) {
            int value = 42;
            FILE *pulse = std::fopen("heartbeat", "w");
            std::fprintf(pulse, "%d", i + value);
            std::fclose(pulse);
            usleep(25000);
        }
    }
    Counter counter{1};
    int count = 41;
    count++;
    count = counter.add(count);
    std::printf("count = %d\n", count);
}
