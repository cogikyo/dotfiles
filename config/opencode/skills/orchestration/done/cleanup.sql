WITH RECURSIVE
tree AS (SELECT id FROM session_v2 WHERE id = ?1 OR parent_id = ?1),
inputs AS (
  SELECT sm.session_id AS sid, CAST(json_extract(c.value, '$.state.input') AS TEXT) AS s
  FROM session_message sm, json_each(sm.data, '$.content') c
  WHERE sm.session_id IN (SELECT id FROM session_v2 WHERE time_updated >= ?2)
    AND json_extract(c.value, '$.type') = 'tool'
    AND CAST(json_extract(c.value, '$.state.input') AS TEXT) LIKE '%/tmp/opencode/%'
),
occ(sid, rest) AS (
  SELECT sid, substr(s, instr(s, '/tmp/opencode/') + 14) FROM inputs
  UNION ALL
  SELECT sid, substr(rest, instr(rest, '/tmp/opencode/') + 14) FROM occ WHERE instr(rest, '/tmp/opencode/') > 0
),
names AS (
  SELECT sid, substr(rest, 1, min(
    CASE instr(rest, '/') WHEN 0 THEN 999 ELSE instr(rest, '/') END,
    CASE instr(rest, ' ') WHEN 0 THEN 999 ELSE instr(rest, ' ') END,
    CASE instr(rest, '"') WHEN 0 THEN 999 ELSE instr(rest, '"') END,
    CASE instr(rest, '\') WHEN 0 THEN 999 ELSE instr(rest, '\') END,
    CASE instr(rest, char(59)) WHEN 0 THEN 999 ELSE instr(rest, char(59)) END,
    CASE instr(rest, '`') WHEN 0 THEN 999 ELSE instr(rest, '`') END,
    CASE instr(rest, '''') WHEN 0 THEN 999 ELSE instr(rest, '''') END,
    CASE instr(rest, ')') WHEN 0 THEN 999 ELSE instr(rest, ')') END,
    CASE instr(rest, '*') WHEN 0 THEN 999 ELSE instr(rest, '*') END,
    CASE instr(rest, '$') WHEN 0 THEN 999 ELSE instr(rest, '$') END,
    CASE instr(rest, ',') WHEN 0 THEN 999 ELSE instr(rest, ',') END,
    CASE instr(rest, '|') WHEN 0 THEN 999 ELSE instr(rest, '|') END
  ) - 1) AS name
  FROM occ
)
SELECT name,
  max(sid IN (SELECT id FROM tree)) AS mine,
  max(sid NOT IN (SELECT id FROM tree)) AS shared
FROM names
WHERE name <> '' AND name NOT LIKE '<%'
GROUP BY name
HAVING mine = 1
ORDER BY shared, name
