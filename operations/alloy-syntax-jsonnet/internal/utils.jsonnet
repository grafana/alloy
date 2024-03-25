{
  // exprMarker is a field name which can be used to mark a Jsonnet object as a
  // River expression literal.
  //
  // The field name *must* be public, otherwise std.prune will remove it and
  // cause the expression literals to be treated as object literals.
  //
  // However, because the field name is public, it is technically possible for
  // it to collide with an object literal key that a user would want to use. We
  // pick a marker name here which is fairly unlikely to appear in a config
  // file to reduce the chance of something being treated as an expr literal.
  exprMarker: '$$__river_jsonnet__expr_literal',
}
