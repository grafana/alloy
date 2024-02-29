import { ObjectField, Value, ValueType } from './types';

/**
 * Returns a native River config representation of the given Value.
 */
export function riverStringify(v: Value): string {
  return riverStringifyImpl(v, 0);
}

function riverStringifyImpl(v: Value, indent: number): string {
  switch (v.type) {
    case ValueType.NULL: {
      return 'null';
    }

    case ValueType.NUMBER: {
      return v.value.toString();
    }

    case ValueType.STRING: {
      return `"${escapeString(v.value)}"`;
    }

    case ValueType.BOOL: {
      if (v.value) {
        return 'true';
      } else {
        return 'false';
      }
    }

    case ValueType.ARRAY: {
      let result = '[';
      v.value.forEach((element, idx) => {
        result += riverStringifyImpl(element, indent);
        if (idx + 1 < v.value.length) {
          result += ', ';
        }
      });
      result += ']';
      return result;
    }

    case ValueType.OBJECT: {
      if (v.value.length === 0) {
        return '{}';
      }

      const partitions = partitionFields(v.value);

      let result = '{\n';

      partitions.forEach((partition) => {
        // Find the maximum field length across all fields in this partition.
        const keyLength = partitionKeyLength(partition);

        return partition.forEach((element) => {
          result += indentLine(indent + 1);
          result += `${partitionKey(element, keyLength)} = ${riverStringifyImpl(element.value, indent + 1)}`;
          result += ',\n';
        });
      });

      result += indentLine(indent) + '}';
      return result;
    }

    case ValueType.FUNCTION: {
      return v.value;
    }

    case ValueType.CAPSULE: {
      return v.value;
    }

    default: {
      return 'null';
    }
  }
}

/**
 * escapeString escapes special characters in a string so they can be printed
 * inside a River string literal.
 */
function escapeString(input: string): string {
  // TODO(rfratto): this should also escape Unicode characters into \u and \U
  // forms.
  return input.replace(/[\b\f\n\r\t\v\0'"\\]/g, (match) => {
    switch (match) {
      case '\b':
        return '\\b';
      case '\f':
        return '\\f';
      case '\n':
        return '\\n';
      case '\r':
        return '\\r';
      case '\t':
        return '\\t';
      case '\v':
        return '\\v';
      case "'":
        return "\\'";
      case '"':
        return '\\"';
      case '\\':
        return '\\\\';
    }
    return '';
  });
}

function indentLine(indentLevel: number): string {
  if (indentLevel === 0) {
    return '';
  }
  return '\t'.repeat(indentLevel);
}

/**
 * partitionFields partitions fields in an object by fields which should have
 * their equal signs aligned.
 *
 * A field which crosses multiple lines (i.e., recursively contains an object
 * with more than one element) will cause a partition break, placing subsequent
 * fields in another partition.
 */
function partitionFields(fields: ObjectField[]): ObjectField[][] {
  const partitions = [];

  let currentPartition: ObjectField[] = [];
  fields.forEach((field) => {
    currentPartition.push(field);

    if (multilinedValue(field.value)) {
      // Fields which cross multiple lines cause a partition break.
      partitions.push(currentPartition);
      currentPartition = [];
    }
  });

  if (currentPartition.length !== 0) {
    partitions.push(currentPartition);
  }

  return partitions;
}

/** multilinedValue returns true if value recrusively crosses multiple lines. */
function multilinedValue(value: Value): boolean {
  switch (value.type) {
    case ValueType.OBJECT:
      // River objects cross more than one line whenever there is at least one
      // element.
      return value.value.length > 0;

    case ValueType.ARRAY:
      // River arrays cross more than one line if any of their elements cross
      // more than one line.
      return value.value.some((v) => multilinedValue(v));
  }

  // Other values never cross line barriers.
  return false;
}

/**
 * partitionKeyLength returns the length of keys within the partition. The
 * length is determined by the longest field name in the partition.
 */
function partitionKeyLength(partition: ObjectField[]): number {
  let keyLength = 0;

  partition.forEach((f) => {
    const fieldLength = partitionKey(f, 0).length;
    if (fieldLength > keyLength) {
      keyLength = fieldLength;
    }
  });

  return keyLength;
}

/**
 * partitionKey returns the text to use to display a key for a field within a
 * partition.
 */
function partitionKey(field: ObjectField, keyLength: number): string {
  let key = field.key;
  if (!validIdentifier(key)) {
    // Keys which aren't valid identifiers should be wrapped in quotes.
    key = `"${key}"`;
  }

  if (key.length < keyLength) {
    return key + ' '.repeat(keyLength - key.length);
  }
  return key;
}

/**
 * validIdentifier reports whether the input is a valid River identifier.
 */
function validIdentifier(input: string): boolean {
  return /^[_a-z][_a-z0-9]*$/i.test(input);
}
