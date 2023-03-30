
def is_new_user(ctx, inputs):
    return ctx.cvt < 600

def is_returning_user(ctx, inputs):
    return not is_new_user(ctx, inputs)

