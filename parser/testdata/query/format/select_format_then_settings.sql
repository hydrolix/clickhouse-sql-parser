-- Origin SQL:
SELECT 1 FORMAT JSON SETTINGS max_threads = 1;

SELECT count() AS n FROM hydro.logs WHERE timestamp >= now() - toIntervalHour(1) GROUP BY app ORDER BY n DESC FORMAT CSVWithNames SETTINGS hdx_query_max_timerange_sec = 86400, hdx_query_max_execution_time = 60;

SELECT 1 SETTINGS a = 1 FORMAT JSON SETTINGS b = 2;

SELECT 1 UNION ALL SELECT 2 FORMAT JSON SETTINGS a = 1;

SELECT 1 FORMAT JSON SETTINGS hdx_query_admin_comment = 'it''s a comment', a = $$dollar quoted$$;

SELECT * FROM test_table FORMAT TSV SETTINGS additional_table_filters = {'test_table': 'status = 1'};


-- Format SQL:
SELECT 1 FORMAT JSON SETTINGS max_threads=1;
SELECT count() AS n FROM hydro.logs WHERE timestamp >= now() - toIntervalHour(1) GROUP BY app ORDER BY n DESC FORMAT CSVWithNames SETTINGS hdx_query_max_timerange_sec=86400, hdx_query_max_execution_time=60;
SELECT 1 SETTINGS a=1 FORMAT JSON SETTINGS b=2;
SELECT 1 UNION ALL SELECT 2 FORMAT JSON SETTINGS a=1;
SELECT 1 FORMAT JSON SETTINGS hdx_query_admin_comment='it''s a comment', a='dollar quoted';
SELECT * FROM test_table FORMAT TSV SETTINGS additional_table_filters={'test_table': 'status = 1'};
