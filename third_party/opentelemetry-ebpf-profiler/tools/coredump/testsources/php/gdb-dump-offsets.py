"""
gdb python script for dumping the PHP engine offsets.
"""

def no_member_to_none(fn):
    """Decorator translating errors about missing field to `None`."""
    def wrap(*args, **kwargs):
        try:
            return fn(*args, **kwargs)
        except gdb.error as e:
            if 'no member named' in str(e):
                return None
            raise
    return wrap

@no_member_to_none
def offset_of(ty, field, ns='struct'):
    return int(gdb.parse_and_eval(f'(uintptr_t)(&(({ns} {ty}*)0)->{field})'))

@no_member_to_none
def size_of(ty, *, ns='struct'):
    return int(gdb.parse_and_eval(f'sizeof({ns} {ty})'))


fields = {
  'zend_executor_globals.current_execute_data': offset_of('_zend_executor_globals', 'current_execute_data'),
  'zend_executor_globals.current_execute_data': offset_of('_zend_executor_globals', 'current_execute_data'),

  'zend_execute_data.opline': offset_of('_zend_execute_data', 'opline'),
  'zend_execute_data.function': offset_of('_zend_execute_data', 'func'),
  'zend_execute_data.this_type_info': offset_of('_zend_execute_data', 'This.u1.type_info'),
  'zend_execute_data.prev_execute_data': offset_of('_zend_execute_data', 'prev_execute_data'),

  'zend_function.common_type': offset_of('_zend_function', 'common.type', ns='union'),
  'zend_function.common_funcname': offset_of('_zend_function', 'common.function_name', ns='union'),
  'zend_function.op_array_filename': offset_of('_zend_function', 'op_array.filename', ns='union'),
  'zend_function.op_array_linestart': offset_of('_zend_function', 'op_array.line_start', ns='union'),
  'zend_function.Sizeof': size_of('_zend_function', ns='union'),

  'zend_string.val': offset_of('_zend_string', 'val'),

  'zend_op.lineno': offset_of('_zend_op', 'lineno'),
}


for field, value in fields.items():
    if value is None:
        print(f"vms.{field}: <field not present>")
    else:
        print(f"vms.{field}: dec={value} hex=0x{value:x}")
