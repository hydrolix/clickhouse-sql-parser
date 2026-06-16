-- Origin SQL:
SELECT 1 UNION ALL (SELECT 2 SETTINGS max_threads = 1)


-- Format SQL:
SELECT 1 UNION ALL (SELECT 2 SETTINGS max_threads=1);
