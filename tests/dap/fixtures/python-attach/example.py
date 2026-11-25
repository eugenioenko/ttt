import ctypes, os, pathlib, sys, time
assert ctypes.CDLL(None).prctl(0x59616d61, os.getppid(), 0, 0, 0) == 0
pathlib.Path("pid").write_text(str(os.getpid()))
print("TARGET_READY", flush=True)
if "--listen" in sys.argv:
    import debugpy
    port = debugpy.listen(("127.0.0.1", 0))[1]
    pathlib.Path("port").write_text(str(port))
    debugpy.wait_for_client()

def run():
  i = 0
  while True:
    value = 40 + 2
    pathlib.Path("heartbeat").write_text(str(i + value))
    i += 1
    time.sleep(0.025)
run()
