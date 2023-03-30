# builtin functions

# graph is a builtin module exposed by the host environment
# graph.nodes = {}
# graph.edges = []


def node(name, operation, conf={}, predicate=None):
    """
    :rtype: str the node name is returned
    :param name: string unique name of the node
    :param operation: string | (ctx, conf) -> any
    :param conf: object, the configuration for the operation
    :param predicate: （ctx, inputs) -> boolean, a predicate
    """
    n = {
        "name": name,
        "operation": operation,
        "conf": conf,
        "predicate": predicate
    }
    message = "node {}".format(n)
    print(message)
    graph.nodes[name] = n
    return name

# predicates
# A predicate is of type (ctx, inputs) -> boolean
def is_runnable(ctx, inputs):
    """
    Whether a node is runnable
    :param ctx: the pipeline context
    :param inputs: the results of all dependency nodes
    """
    # The first input is a FetchRanking node, and the result is a Ranking object.
    return inputs[0].size > 0

def edge(src, dst):
    message = "{} -> {}".format(src, dst)
    print(message)
    graph.edges.append((src, dst))

def sequence(*nodes):
    size = len(nodes)
    last = None
    for i in range(0, size):
        src = nodes[i]
        apply = node("{src}:apply".format(src=src), 'apply', None, is_runnable)
        edge(src, apply)
        last = apply
        if i < size - 1:
            dst = nodes[i + 1]
            edge(apply, "{}:apply".format(dst))
    return last

def before(source, *nodes):
    size = len(nodes)
    last = None
    for i in range(0, size):
        src = nodes[i]
        edge(source, src)
        last = src
    return last
