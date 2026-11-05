-- Origin SQL:
SELECT 1 SETTINGS limit = 5, offset = 10, final = 1;
SELECT 1 FORMAT JSON SETTINGS limit = 5;


-- Beautify SQL:
SELECT
  1
SETTINGS
  limit=5,
  offset=10,
  final=1;
SELECT
  1
FORMAT JSON
SETTINGS
  limit=5;
