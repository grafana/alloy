/*
 * A Stmt is a statement found within the body of a file or a block.
 */
export type Stmt = BlockStmt | AttrStmt;

/*
 * A Body is a list of statements.
 */
export type Body = Stmt[];

/*
 * StmtType enumerates the potential values for the type field in Stmt types.
 */
export enum StmtType {
  /** BlockStmt */
  BLOCK = 'block',
  /** AttrStmt */
  ATTR = 'attr',
}

/**
 * BlockStmt is a named body with an optional label.
 */
export interface BlockStmt {
  type: StmtType.BLOCK;
  name: string;
  label?: string;
  body: Body;
}

/**
 * AttrStmt sets a named value in a body.
 */
export interface AttrStmt {
  type: StmtType.ATTR;
  name: string;
  value: Value;
}

/**
 * Value represents an Alloy value.
 */
export type Value =
  | NullValue
  | NumberValue
  | StringValue
  | BoolValue
  | ArrayValue
  | ObjectValue
  | FunctionValue
  | CapsuleValue;

/**
 * ValueType enumerates the possible values for the type field in a Value
 * interface.
 */
export enum ValueType {
  NULL = 'null',
  NUMBER = 'number',
  STRING = 'string',
  BOOL = 'bool',
  ARRAY = 'array',
  OBJECT = 'object',
  FUNCTION = 'function',
  CAPSULE = 'capsule',
}

/**
 * NullValue represents an Alloy null.
 */
export interface NullValue {
  type: ValueType.NULL;
}

/**
 * NumberValue represents an Alloy number.
 */
export interface NumberValue {
  type: ValueType.NUMBER;
  value: number;
}

/**
 * StringValue represents an Alloy string.
 */
export interface StringValue {
  type: ValueType.STRING;
  value: string;
}

/**
 * BoolValue represents an Alloy bool.
 */
export interface BoolValue {
  type: ValueType.BOOL;
  value: boolean;
}

/**
 * ArrayValue represents an Alloy array. Elements are Alloy Values of any type.
 */
export interface ArrayValue {
  type: ValueType.ARRAY;
  value: Value[];
}

/**
 * ObjectValue represents an Alloy object. Each field has a name and a value,
 * similar to an attribute. It is invalid for an ObjectValue to define the same
 * field twice.
 */
export interface ObjectValue {
  type: ValueType.OBJECT;
  value: ObjectField[];
}

/**
 * ObjectField represents a field within an Alloy object value. It is similar to
 * an attribute, having a key and a value.
 */
export interface ObjectField {
  key: string;
  value: Value;
}

/**
 * FunctionValue represents an Alloy function. The value field is literal text
 * to display which represents the function.
 */
export interface FunctionValue {
  type: ValueType.FUNCTION;
  value: string;
}

/**
 * CapsuleValue represents an Alloy capsule. The value field is literal text to
 * display which represents the capsule.
 */
export interface CapsuleValue {
  type: ValueType.CAPSULE;
  value: string;
}
