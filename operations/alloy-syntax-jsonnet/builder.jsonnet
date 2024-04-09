local utils = import './internal/utils.jsonnet';

{
  // attr returns the field name that should be used for Alloy attributes.
  attr(name):: name,

  // block returns the field name that should be used for Alloy blocks.
  block(name, label='', index=0)::
    if label == ''
    then ('block %s %d' % [name, index])
    else ('block %s %s %d' % [name, label, index]),

  // expr returns an object which represents a literal Alloy expression.
  expr(lit):: {
    // We need to use a special marker field to indicate that this object is an
    // expression, otherwise manifest.jsonnet would treat it as an object
    // literal.
    //
    // This field *must* be public. See utils.exprMarker for more information.
    [utils.exprMarker]: lit,
  },
}
