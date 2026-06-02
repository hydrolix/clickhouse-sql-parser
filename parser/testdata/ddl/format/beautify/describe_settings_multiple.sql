-- Origin SQL:
DESCRIBE foo SETTINGS describe_compact_output=1, describe_include_subcolumns=1


-- Beautify SQL:
DESCRIBE foo
SETTINGS
  describe_compact_output=1,
  describe_include_subcolumns=1;
