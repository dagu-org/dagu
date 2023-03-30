load("internal/pipeline/data/dag.star", "node", "edge", "is_runnable", "sequence", "before")
load("internal/pipeline/data/ops.star", "content_holdback", "ranking", "batch_ranking", "realtime_ranking",
     "container_pinning", "apply_node")
load("internal/pipeline/data/predicates.star", "is_returning_user", "is_new_user")


def container_pinning_i18n_predicate(req):
    return req.first_seen - time.now() > time.day(3)

def main(args):
    request_validation = node("request_validation", "validation")
    container_recall = realtime_ranking("container_recall",
                                        "container_recall_only_recommender",
                                        app="CONTAINER_RECALL",
                                        predicate=lambda req: len(req.containers) > 40)
    container_pin = container_pinning("container_pinning_i18n", ["featured", "recommended_for_you"], container_pinning_i18n_predicate)
    apply_container_recall = apply_node("container_recall")
    before(request_validation, container_recall, container_pin)
    edge(container_recall, apply_container_recall)
    edge(container_pin, apply_container_recall)
    stage1 = apply_container_recall

    aggregate_ranking_trending = realtime_ranking("aggregate_ranking_trending", "aggregate_ranking_trending")
    featured_rank_default = realtime_ranking("featured_rank_default", "new_user_default_featured_recommender")
    content_recall_genesis = realtime_ranking("content_recall_genesis", "GENESIS")
    content_rank_default = realtime_ranking("content_rank_default", "content_rank_default")
    content_rank_context = realtime_ranking("content_rank_context", "new_user_content_recommender")
    content_rank_specific = realtime_ranking("content_rank_specific", "returning_user_content_recommender")
    before(stage1, aggregate_ranking_trending, featured_rank_default, content_recall_genesis, content_rank_default,
             content_rank_context, content_rank_specific)
    stage2 = sequence(aggregate_ranking_trending, featured_rank_default, content_recall_genesis, content_rank_default,
             content_rank_context, content_rank_specific)

    promoter = node("content_promotion", "promoter")
    container_rank_default = realtime_ranking("container_rank_default", "container_rank_default")
    # sequence(promoter, container_rank_default)
    apply_promoter = apply_node(promoter)
    apply_container_rank = apply_node(container_rank_default)
    before(stage2, promoter, container_rank_default)
    edge(promoter, apply_promoter)
    edge(container_rank_default, apply_container_rank)
    content_holdback = node("content_holdback", "content_holdback", ["0123"])
    bd_levers = node("matrix_bd_levers", "bd_levers")
    container_float = node("container_floating_i18n", "container_floating")
    # edge(stage2, "content_promotion:apply")
    edge("content_promotion:apply", content_holdback)
    edge(content_holdback, "container_rank_default:apply")
    edge("container_rank_default:apply", bd_levers)
    edge(bd_levers, container_float)
    stage3 = container_float

    image_rank = realtime_ranking("image_rank_specific", "cb_image_rank_with_stats_v2_recommender")
    container_truncate = node("container_truncate", "container_truncate")
    content_truncate = node("content_truncate", "content_truncate")
    dedupe = node("dedupe", "dedupe")
    apply_image_rank = apply_node(image_rank)
    before(stage3, image_rank)
    edge(image_rank, apply_image_rank)
    edge(stage3, container_truncate)
    edge(container_truncate, content_truncate)
    edge(content_truncate, dedupe)
    edge(dedupe, apply_image_rank)
    return 0
