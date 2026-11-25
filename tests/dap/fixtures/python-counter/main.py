def increment(value):
    return value + 1

def run():
    count = 41
    count += 1
    count = increment(count)
    print("count =", count)

run()
