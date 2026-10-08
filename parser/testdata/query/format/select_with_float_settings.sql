-- Origin SQL:
SELECT 1 SETTINGS a = 0.5, b = -0.5, c = +0.5, d = .5, e = 1., f = 1.5e-3;
SELECT 1 SETTINGS a = 0.5, b = 1, c = 'x', d = TRUE;
SELECT 1 FORMAT JSON SETTINGS x = 0.5;


-- Format SQL:
SELECT 1 SETTINGS a=0.5, b=-0.5, c=+0.5, d=.5, e=1., f=1.5e-3;
SELECT 1 SETTINGS a=0.5, b=1, c='x', d=TRUE;
SELECT 1 FORMAT JSON SETTINGS x=0.5;
