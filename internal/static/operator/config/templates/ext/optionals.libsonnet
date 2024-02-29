{
  // string returns null if value is an empty length string, otherwise
  // returns the input.
  string(value)::
    if value == null then null
    else
      assert std.isString(value) : 'string must only be called with strings';
      if std.length(value) == 0 then null else value,

  // number returns null if value is 0, otherwise returns value.
  number(value)::
    if value == null then null
    else
      assert std.isNumber(value) : 'number must only be called with numbers';
      if value == 0 then null else value,

  // bool returns a value only if the value is present, and not equal to the default. otherwise returns null.
  bool(value, defaultValue = false)::
    if value == null then null
    else
      assert std.isBoolean(value) : 'bool must only be called with booleans';
      if value == defaultValue then null else value,

  // object returns null if there are no keys in the object.
  object(value)::
    if value == null then null
    else
      assert std.isObject(value) : 'object must only be called with objects';
      if std.length(value) == 0 then null else value,

  // array returns null if there are no elements in the array.
  array(value)::
    if value == null then null
    else
      assert std.isArray(value) : 'array must only be called with arrays';
      if std.length(value) == 0 then null else value,

  // trim will recursively traverse through value and remove all fields
  // from value that have a value of null.
  trim(value):: std.native('trimOptional')(value),
}
