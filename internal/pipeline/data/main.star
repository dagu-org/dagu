
def fibonacci(n):
   res = list(range(n))
   for i in res[2:]:
       res[i] = res[i-2] + res[i-1]
   return res

def main(n):
    start = time.now()
    res = fibonacci(n)
    end = time.now()
    print("duration: %s" % (end - start))
    # message = "fib(%d) = %s" % (n, res)
    message = "fib({n}) = {res}".format(n = n, res = res)
    print(message)
    return 0
