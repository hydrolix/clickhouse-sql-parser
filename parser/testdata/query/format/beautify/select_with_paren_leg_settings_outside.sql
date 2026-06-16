-- Origin SQL:
SELECT 1 UNION ALL (SELECT 2) SETTINGS max_threads = 1


-- Beautify SQL:
SELECT
  1
UNION ALL
(SELECT
  2)
SETTINGS
  max_threads=1;
