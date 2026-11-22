# Health import (Apple Health → RepLog)

RepLog can take sleep, resting heart rate, HRV and step counts from a
phone or watch and store them as `bio_samples` rows with
`source = 'watch_import'`. They show up next to the athlete's other
readings and inform the coach. **They never change a plan**: there is no
auto-deload or readiness-driven change anywhere in RepLog. The coach
decides.

Two ways to send data, both to the same endpoint:

- **Health Auto Export** (iOS app, paid tier for automations) posts
  everything on a schedule with no glue. Easiest for sleep.
- **An iOS Shortcut** run as a daily personal automation. Free, a little
  more setup, best for resting HR, HRV and steps.

## The endpoint

```
POST /api/athletes/{id}/bio-samples/import
Content-Type: application/json
Authorization: Bearer rlcst_...
```

`{id}` is the athlete's ID (the number in `/athletes/{id}` in the app).

### Authentication

Phones can't hold a browser session, so use a **service token**:

1. An admin opens **Users → (the user) → Service tokens** and issues one.
   The plaintext `rlcst_...` value is shown once; copy it into the
   Shortcut or Health Auto Export header.
2. The token acts as that user and sees exactly what they would: their
   own linked athlete, plus (for a coach) the athletes they coach. An
   adult who logs their own training issues a token on their own user;
   a parent sending a kid's watch data uses a token on the coach user.
   Any other athlete returns `403`.
3. Revoke the token from the same screen if a phone is lost.

A signed-in browser session also works (same access rules), which is
handy for testing with the API docs at `/api/docs`.

### Request body

Send `samples`, Health Auto Export's `data`, or both.

```json
{
  "samples": [
    { "metric": "resting_hr", "value": 52, "unit": "bpm", "recorded_at": "2026-11-21T06:30:00-05:00" },
    { "metric": "hrv",        "value": 61, "unit": "ms",  "recorded_at": "2026-11-21T06:30:00-05:00" },
    { "metric": "steps",      "value": 9123,              "recorded_at": "2026-11-21" },
    { "metric": "sleep_hours","value": 7.4, "unit": "h",  "recorded_at": "2026-11-21" }
  ]
}
```

| Metric (canonical) | Also accepted | Stored unit | Range |
|---|---|---|---|
| `sleep_hours` | `sleep`, `sleep_analysis` | h (send `"unit":"min"` to convert) | 0–24 |
| `resting_hr` | `resting_heart_rate` | bpm | 20–250 |
| `hrv` | `heart_rate_variability`, `heart_rate_variability_sdnn` | ms | 0–500 |
| `steps` | `step_count` | count | 0–200 000 |

Names are case-insensitive; spaces and dashes count as underscores, so
`Resting Heart Rate` works.

`recorded_at` accepts ISO 8601 (`2026-11-21T06:30:00Z`,
`2026-11-21T06:30:00-05:00`), Health Auto Export's
`2026-11-21 06:30:00 -0500`, a zone-less `2026-11-21 06:30:00`, or a bare
date `2026-11-21` for daily totals. Zone-less values are read in the
sending user's timezone (Preferences). Everything is stored in UTC.

Limits: 5000 readings and 5 MiB per request. Split a long back-fill into
several posts.

### Idempotency

A reading is identified by **(athlete, metric, recorded_at)**. If that
key is already stored, whether imported earlier or entered by hand, the
reading is counted as a duplicate and skipped. Sending the last seven days
every morning is therefore safe, and so is re-running a failed automation.

### Response

`200` with a summary. Bad readings don't fail the batch:

```json
{
  "received": 6,
  "inserted": 4,
  "duplicates": 1,
  "skipped": 1,
  "errors": [{ "index": 3, "metric": "hrv", "reason": "hrv value 9000 is outside 0–500 ms" }],
  "ignored_metrics": ["active_energy"]
}
```

`errors` lists the first 50 skipped readings by their position in the
request. `ignored_metrics` names whole Health Auto Export series RepLog
doesn't import. They are not errors.

Other statuses: `400` (not JSON, or no readings), `401` (missing or bad
token), `403` (the token can't see this athlete), `413` (too big).

### Try it

```bash
curl -sS -X POST "https://replog.example.com/api/athletes/3/bio-samples/import" \
  -H "Authorization: Bearer $REPLOG_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{"samples":[{"metric":"resting_hr","value":52,"recorded_at":"2026-11-21"}]}'
```

Run it twice: the second response has `"duplicates": 1`.

## Recipe: Health Auto Export

[Health Auto Export](https://www.healthyapps.dev/) reads Apple Health and
posts JSON on a schedule. RepLog accepts its REST API payload as-is. Menu
names below may differ a little between app versions.

1. In Health Auto Export, create a new **Automation** of type
   **REST API**.
2. **URL:** `https://<your-replog>/api/athletes/<id>/bio-samples/import`
3. **Headers:** add `Authorization` = `Bearer rlcst_...`. JSON content
   type is the default.
4. **Data type:** Health Metrics. Select **Resting Heart Rate**,
   **Heart Rate Variability**, **Step Count** and **Sleep Analysis**.
   Anything else you tick is reported back as `ignored_metrics`.
5. **Format:** JSON. Turn on **summarize / aggregate data** with a
   **daily** interval, so you get one sleep total per night (the
   `asleep` / `totalSleep` fields RepLog reads) and one step total per day.
6. **Date range:** the last 7 days (overlap is fine; see
   [Idempotency](#idempotency)). **Sync cadence:** daily, or every few
   hours.
7. Run it once by hand and check the response in the automation's log.

Health Auto Export payloads look like this; you don't need to build
them yourself:

```json
{
  "data": {
    "metrics": [
      { "name": "resting_heart_rate", "units": "count/min",
        "data": [{ "date": "2026-11-21 00:00:00 -0500", "qty": 52 }] },
      { "name": "sleep_analysis", "units": "hr",
        "data": [{ "date": "2026-11-21 00:00:00 -0500", "asleep": 7.1, "inBed": 7.8 }] }
    ]
  }
}
```

For sleep, RepLog stores `asleep` (falling back to `totalSleep`) in hours.
Non-aggregated sleep segments, which carry no `asleep` / `totalSleep`
total, are skipped and listed in `errors`. Turn aggregation on.

## Recipe: iOS Shortcut

A daily personal automation that sends last night's resting HR, HRV and
yesterday's step total. Build it once in the Shortcuts app.

1. **Shortcuts → Automation → New → Time of Day**, for example 9:00 daily,
   set to **Run Immediately**.
2. Add **Find Health Samples**: Type **Resting Heart Rate**, Start Date
   *is in the last* **1 day**, sort by Start Date *Latest First*, Limit **1**.
   Then **Get Details of Health Samples → Value** (call it *RHR*) and
   **Get Details → Start Date**, run through **Format Date → ISO 8601**
   (call it *RHR Date*).
3. Repeat step 2 for **Heart Rate Variability** (*HRV*, *HRV Date*).
4. **Find Health Samples**: Type **Steps**, Start Date *is yesterday*, no
   limit, then **Calculate Statistics → Sum** (*Steps*). Use
   **Format Date** on *Current Date* minus one day as `yyyy-MM-dd`
   (*Day*).
5. Add a **Text** action with the body. Insert the variables where shown:

   ```
   {"samples":[
     {"metric":"resting_hr","value":RHR,"recorded_at":"RHR Date"},
     {"metric":"hrv","value":HRV,"recorded_at":"HRV Date"},
     {"metric":"steps","value":Steps,"recorded_at":"Day"}
   ]}
   ```

6. Add **Get Contents of URL**:
   - URL `https://<your-replog>/api/athletes/<id>/bio-samples/import`
   - Method **POST**
   - Headers: `Authorization` = `Bearer rlcst_...`,
     `Content-Type` = `application/json`
   - Request Body **File**, set to the Text from step 5.
7. Optionally **Show Notification** with the result while testing, then
   remove it.

Sleep from a Shortcut: Find Health Samples of **Sleep Analysis** for the
last night, keep the asleep stages, **Get Details → Duration**, sum them,
and send the total in minutes with `"unit":"min"`. RepLog converts it to
hours. If the duration arithmetic gets fiddly, use Health Auto Export for
sleep.

A missing reading (no HRV last night) leaves an empty value in the JSON
and the whole request fails to parse. Either keep the automation to
metrics the watch always records, or add an **If** around each line.

## Where the data shows up

Imported readings appear on the athlete's bio-sample list
(`GET /api/athletes/{id}/bio-samples`), tagged `watch_import`. Rows are
append-only, like manual readings. They are context for the coach's
review. Nothing in RepLog acts on them automatically.
