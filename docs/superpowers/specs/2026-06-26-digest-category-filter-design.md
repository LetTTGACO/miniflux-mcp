# Digest Category Filter Design

## Goal

Update `get_daily_digest` so callers can include multiple categories and exclude categories when building a digest.

## Tool Contract

`get_daily_digest` will replace the current single `category_id` argument with `category_ids`, an optional array of numeric category IDs. It will also add `exclude_category_ids`, an optional array of numeric category IDs to omit.

The existing `category_id` argument is intentionally removed because this server is currently used by one caller and does not need backward compatibility for this parameter.

## Filtering Semantics

If `category_ids` is omitted or empty, the digest initially considers all categories.

If `category_ids` is provided, the digest initially considers only entries whose feed category ID is in that set.

If `exclude_category_ids` is provided, entries whose feed category ID is in that set are removed from the digest after the include set is applied.

When both arguments are present, the effective category set is:

```text
real_category_ids = category_ids - exclude_category_ids
```

For example, `category_ids: [1, 2, 4]` and `exclude_category_ids: [4]` returns only categories `1` and `2`.

## Implementation

The pinned Miniflux Go client exposes a single `CategoryID` filter for entry queries, not a multiple-category or negative-category filter. To avoid missing entries, `get_daily_digest` will query entries using the existing optional time, status, feed, limit, and ordering filters, then apply include/exclude category filtering inside the MCP server before building digest entries and acknowledgement IDs.

The response `count` and `ack_entry_ids` will reflect entries remaining after category filtering. The response `total` will continue to report the Miniflux API total for the upstream query because the API returns that value before local filtering.

## Testing

Add focused tests for:

- The `get_daily_digest` schema exposes `category_ids` and `exclude_category_ids`, and no longer exposes `category_id`.
- Include and exclude category filters combine as `category_ids - exclude_category_ids`.
- Exclude-only filtering removes matching categories from an all-category digest.

Update README digest argument documentation to match the new contract.
