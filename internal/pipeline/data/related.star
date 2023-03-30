load("internal/pipeline/data/dag.star", "node", "edge", "is_runnable", "sequence")
load("internal/pipeline/data/ops.star", "content_holdback", "ranking", "batch_ranking", "realtime_ranking")
load("internal/pipeline/data/predicates.star", "is_returning_user")

def apply_node(src):
    return node("{src}:apply".format(src=src), 'apply', None, None)

def main(args):
    rank_int = ranking(
        "related_rank_intl",
        {
            "model_app": "RELATED_RANK",
            "model_state": "RELEASE",
            "id_selectors": [
                "CONTENT_ID"
            ],
            "serving_mode": "BATCH"
        },
        None
    )
    related_rank_specific = batch_ranking(
        'related_rank_specific',
        "RELATED_RANK",
        "RELEASE",
        id_selectors = [
            "COUNTRY_ID",
            "CONTENT_ID"
        ]
    )
    related_rank_specific_rt = ranking(
        'related_rank_specific_rt',
        {
            "model_app": "RELATED_RANK",
            "model_state": "RELEASE",
            "model_name": "ymal_recommender",
            "id_selectors": [],
            "serving_mode": "REALTIME"
        },
        None
    )
    related_rank_specific_rt_returning_user = realtime_ranking(
        'related_rank_specific_rt_returning_user',
        "RELATED_RANK",
        "RELEASE",
        "ymal_recommender_returning_user",
        is_returning_user
    )
    content_holdback_op = content_holdback('content_holdback')

    # apply_rank_int = apply_node(rank_int)
    # apply_related_rank_specific = apply_node(related_rank_specific)
    # apply_related_rank_specific_rt = apply_node(related_rank_specific_rt)
    # apply_related_rank_specific_rt_returning_user = apply_node(related_rank_specific_rt_returning_user)
    # edge(rank_int, apply_rank_int)
    # edge(related_rank_specific, apply_rank_int)
    # edge(related_rank_specific_rt, apply_rank_int)
    # edge(related_rank_specific_rt_returning_user, apply_rank_int)
    #
    # edge(apply_rank_int, apply_related_rank_specific)
    # edge(apply_related_rank_specific, apply_related_rank_specific_rt)
    # edge(apply_related_rank_specific_rt, apply_related_rank_specific_rt_returning_user)
    # edge(apply_related_rank_specific_rt_returning_user, content_holdback_op)
    last = sequence(rank_int, related_rank_specific, related_rank_specific_rt, related_rank_specific_rt_returning_user)
    edge(last, content_holdback_op)
    return 0
