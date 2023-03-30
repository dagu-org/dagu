load("internal/pipeline/data/dag.star", "node")

def content_holdback(name, conf={}, predicate=None):
    return node(
        name,
        'content_holdback',
        conf,
        predicate
    )

def ranking(name, conf={}, predicate=None):
    return node(
        name,
        'ranking',
        conf,
        predicate
    )

def batch_ranking(name, app, state, id_selectors=[], predicate=None):
    return ranking(
        name,
        {
            "model_app": app,
            "model_state": state,
            "id_selectors": id_selectors,
            "serving_mode": "BATCH"
        },
        predicate
    )

def realtime_ranking(name, model, app="ranking", state="RELEASE", predicate=None):
    return ranking(
        name,
        {
            "model_app": app,
            "model_state": state,
            "model_name": model,
            "serving_mode": "REALTIME"
        },
        predicate
    )

def container_pinning(name, containers, predicate=None):
    return node(
        name,
        'container_pin',
        {"containers": containers},
        predicate
    )

def apply_node(src):
    return node("{src}:apply".format(src=src), 'apply', None, None)
