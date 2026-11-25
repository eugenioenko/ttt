using System;
using System.IO;
using System.Runtime.InteropServices;
using System.Threading;

class Fixture
{
    static int Increment(int value)
    {
        return value + 1;
    }

    static void Main(string[] args)
    {
        if (args.Length > 0)
        {
            Service();
            return;
        }
        int count = 41;
        count++;
        count = Increment(count);
        Console.WriteLine($"count = {count}");
    }

    [DllImport("libc")]
    static extern int getppid();

    [DllImport("libc")]
    static extern int prctl(int option, nuint pid, nuint arg3, nuint arg4, nuint arg5);

    static void Service()
    {
        if (prctl(0x59616d61, (nuint)getppid(), 0, 0, 0) != 0)
            throw new InvalidOperationException("Unable to authorize fixture debugger");
        File.WriteAllText("pid", Environment.ProcessId.ToString());
        Console.WriteLine("TARGET_READY");
        for (int i = 0; ; i++)
        {
            int value = 42;
            File.WriteAllText("heartbeat", (i + value).ToString());
            Thread.Sleep(25);
        }
    }
}
