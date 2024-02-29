// secrets.libsonnet provides utilities for interacting with secrets that are
// both loaded into memory when building the configuration (for confguring
// values that otherwise cannot be read from a file) and mounted into pods (for
// values that *can* be read from a file.)
//
// Since type information is lost in the conversion to Jsonnet, we have to
// specify if a selector is specifically for a secret, config map, or either.

local keyValue(key) =
  if key == null then null
  else std.native('secretLookup')(key);

local keyPath(key) =
  if key == null then null
  else std.native('secretPath')(key);

// functions to get the key for a given selector.
local keys = {
  forSecret(namespace, selector)::
    if selector == null then null
    else '/secrets/%s/%s/%s' % [
      namespace,
      selector.LocalObjectReference.Name,
      selector.Key,
    ],

  forConfigMap(namespace, selector)::
    if selector == null then null
    else '/configMaps/%s/%s/%s' % [
      namespace,
      selector.LocalObjectReference.Name,
      selector.Key,
    ],

  forSelector(namespace, selector)::
    if selector == null then null
    else if selector.Secret != null then $.forSecret(namespace, selector.Secret)
    else if selector.ConfigMap != null then $.forConfigMap(namespace, selector.ConfigMap),
};

{
  // valueForSecret gets the cached value of a SecretKeySelector.
  valueForSecret(namespace, selector)::
    keyValue(keys.forSecret(namespace, selector)),

  // valueForConfigMap gets the cached value of a ConfigMapKeySelector.
  valueForConfigMap(namespace, selector)::
    keyValue(keys.forConfigMap(namespace, selector)),

  // valueForSelector gets the cached value of a SecretOrConfigMap.
  valueForSelector(namespace, selector)::
    keyValue(keys.forSelector(namespace, selector)),

  // pathForSecret gets the path on disk for a SecretKeySelector.
  pathForSecret(namespace, selector)::
    keyPath(keys.forSecret(namespace, selector)),

  // pathForConfigMap gets the path on disk for a ConfigMapKeySelector.
  pathForConfigMap(namespace, selector)::
    keyPath(keys.forConfigMap(namespace, selector)),

  // pathForSelector gets the path on disk for a SecretOrConfigMap.
  pathForSelector(namespace, selector)::
    keyPath(keys.forSelector(namespace, selector)),
}
