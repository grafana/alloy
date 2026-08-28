local alloy = import './main.libsonnet';

// The expectations below have to have sorted fields, since Jsonnet won't give
// you the fields back in the original order.

local tests = [
  {
    name: 'Attributes',
    input: {
      array_attr: ['Hello', 50, false],
      bool_attr: true,
      number_attr: 1234,
      object_attr: { a: 5, b: 6 },
      string_attr: 'Hello, world!',
    },
    expect: |||
      array_attr = ["Hello", 50, false]
      bool_attr = true
      number_attr = 1234
      object_attr = {
        "a" = 5,
        "b" = 6,
      }
      string_attr = "Hello, world!"
    |||,
  },
  {
    name: 'Exprs',
    input: {
      expr_attr: alloy.expr('prometheus.remote_write.default.receiver'),
    },
    expect: |||
      expr_attr = prometheus.remote_write.default.receiver
    |||,
  },
  {
    name: 'Blocks',
    input: {
      [alloy.block('labeled_block', 'foobar')]: {
        attr_1: 15,
        attr_2: 30,
      },
      [alloy.block('unlabeled_block')]: {
        attr_1: 15,
        attr_2: 30,
      },
    },
    expect: |||
      labeled_block "foobar" {
        attr_1 = 15
        attr_2 = 30
      }
      unlabeled_block {
        attr_1 = 15
        attr_2 = 30
      }
    |||,
  },
  {
    name: 'Ordered blocks',
    input: {
      [alloy.block('labeled_block', 'foobar', index=1)]: {
        attr_1: 15,
        attr_2: 30,
      },
      [alloy.block('unlabeled_block', index=0)]: {
        attr_1: 15,
        attr_2: 30,
      },
    },
    expect: |||
      unlabeled_block {
        attr_1 = 15
        attr_2 = 30
      }
      labeled_block "foobar" {
        attr_1 = 15
        attr_2 = 30
      }
    |||,
  },
  {
    name: 'Nested blocks',
    input: {
      [alloy.block('outer.block')]: {
        attr_1: 15,
        attr_2: 30,
        [alloy.block('inner.block')]: {
          attr_3: 45,
          attr_4: 60,
        },
      },
    },
    expect: |||
      outer.block {
        attr_1 = 15
        attr_2 = 30
        inner.block {
          attr_3 = 45
          attr_4 = 60
        }
      }
    |||,
  },
  {
    name: 'Complex example',
    input: {
      attr_1: 'Hello, world!',
      [alloy.block('some_block', 'foobar')]: {
        attr_1: [0, 1, 2, 3],
        attr_2: { first_name: 'John', last_name: 'Smith' },
        expr: alloy.expr('sys.env("HOME")'),
      },
    },
    expect: |||
      attr_1 = "Hello, world!"
      some_block "foobar" {
        attr_1 = [0, 1, 2, 3]
        attr_2 = {
          "first_name" = "John",
          "last_name" = "Smith",
        }
        expr = sys.env("HOME")
      }
    |||,
  },
  {
    name: 'List of blocks',
    input: {
      attr_1: 'Hello, world!',

      [alloy.block('outer_block')]: {
        attr_1: 53,
        [alloy.block('inner_block', 'labeled')]: [
          { bool: true },
          { bool: false },
        ],
        [alloy.block('inner_block', 'other_label')]: [
          { bool: true },
          { bool: false },
        ],
      },
    },
    expect: |||
      attr_1 = "Hello, world!"
      outer_block {
        attr_1 = 53
        inner_block "labeled" {
          bool = true
        }
        inner_block "labeled" {
          bool = false
        }
        inner_block "other_label" {
          bool = true
        }
        inner_block "other_label" {
          bool = false
        }
      }
    |||,
  },
  {
    name: 'Indented literals',
    input: {
      attr_1: alloy.expr('array.concat([%s])' % alloy.manifestAlloyValue({ hello: 'world' })),
    },
    expect: |||
      attr_1 = array.concat([{
        "hello" = "world",
      }])
    |||,
  },
  {
    name: 'Pruned expressions',
    input: std.prune({
      expr: alloy.expr('sys.env("HOME")'),
    }),
    expect: |||
      expr = sys.env("HOME")
    |||,
  },
];

std.map(function(test) (
  assert alloy.manifestAlloy(test.input) == test.expect : (
    |||
      %s FAILED

      EXPECT
      ======
      %s

      ACTUAL
      ======
      %s
    ||| % [test.name, test.expect, alloy.manifestAlloy(test.input)]
  );
  '%s: PASS' % test.name
), tests)
