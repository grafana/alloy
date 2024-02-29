{
  // honorLabels calculates the value for honor_labels based on the value
  // for honor and override, both of which should be bools.
  honorLabels(honor, override):: if honor && override then false else honor,

  // honorTimestamps returns a bool or a null based on the value of honor
  // and override. honor should be either a bool or a null. override should be
  // a bool.
  honorTimestamps(honor, override)::
    if honor == null && !override then null
    else (
      local shouldHonor = if honor != null then honor else false;
      shouldHonor && !override
    ),

  // limit calculates a limit based on the user-provided limit and an optional
  // enforced limit, which may be null.
  limit(user, enforced)::
    if enforced == null then user else (
      if (user < enforced) && (user != 0) && (enforced == 0)
      then user
      else enforced
    ),

  // namespacesFromSelector returns a list of namespaces to select in
  // kubernetes_sd_config based on the given NamespaceSelector selector,
  // string namespace, and whether selectors should be ignored.
  namespacesFromSelector(selector, namespace, ignoreSelectors)::
    if ignoreSelectors then [namespace]
    else if selector.Any == true then []
    else if std.length($.array(selector.MatchNames)) == 0 then
      // If no names are manually provided, then the default behavior is to only
      // look in the current namespace.
      [namespace]
    else $.array(selector.MatchNames),

  // sanitize sanitizes text for label safety.
  sanitize(text):: std.native('sanitize')(text),

  // intOrString returns the string value of *intstr.IntOrString.
  intOrString(obj)::
    if obj == null then ''
    else if obj.StrVal != '' then obj.StrVal
    else if obj.IntVal != 0 then std.toString(obj.IntVal)
    else '',

  // array treats val is a Go slice, where null is the same as an empty array.
  array(val):: if val != null then val else [],
}
