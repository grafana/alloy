"""
gdb python script for dumping Python offsets.
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
def offset_of(ty, field):
    for ns in ['', 'struct ', 'union ']:
        try:
            return int(gdb.parse_and_eval(f'(uintptr_t)(&(({ns}{ty}*)0)->{field})'))
        except gdb.error:
            pass
    return None

@no_member_to_none
def size_of(ty):
    for ns in ['', 'struct ', 'union ']:
        try:
            return int(gdb.parse_and_eval(f'sizeof({ns} {ty})'))
        except gdb.error:
            pass
    return None

fields = {
    'PyTypeObject.BasicSize': offset_of('_typeobject', 'tp_basicsize'),
    'PyTypeObject.Members': offset_of('_typeobject', 'tp_members'),

    'PyMemberDef.Sizeof': size_of('PyMemberDef'),
    'PyMemberDef.Name': offset_of('PyMemberDef', 'name'),
    'PyMemberDef.Offset': offset_of('PyMemberDef', 'offset'),

    'PyASCIIObject.Data': offset_of('PyASCIIObject', 'data'),

    'PyCodeObject.Sizeof': size_of('PyCodeObject'),
    'PyCodeObject.ArgCount': offset_of('PyCodeObject', 'co_argcount'),
    'PyCodeObject.KwOnlyArgCount': offset_of('PyCodeObject', 'co_kwonlyargcount'),
    'PyCodeObject.Flags': offset_of('PyCodeObject', 'co_flags'),
    'PyCodeObject.FirstLineno': offset_of('PyCodeObject', 'co_firstlineno'),
    'PyCodeObject.Filename': offset_of('PyCodeObject', 'co_filename'),
    'PyCodeObject.Name': offset_of('PyCodeObject', 'co_name'),
    'PyCodeObject.Lnotab': offset_of('PyCodeObject', 'co_lnotab'),
    'PyCodeObject.Linetable': offset_of('PyCodeObject', 'co_linetable'),
    'PyCodeObject.QualName': offset_of('PyCodeObject', 'co_qualname'),

    'PyVarObject.ObSize': offset_of('PyVarObject', 'ob_size'),

    'PyBytesObject.Sizeof': size_of('PyBytesObject'),

    'PyThreadState.Frame': offset_of('_ts', 'frame'),
    'PyThreadState.cFrame': offset_of('_ts', 'current_frame'),

    'PyFrameObject.Back': offset_of('PyFrameObject', 'f_back'),
    'PyFrameObject.Code': offset_of('PyFrameObject', 'f_code'),
    'PyFrameObject.LastI': offset_of('PyFrameObject', 'f_lasti'),

    'PyInterpreterFrame.Previous': offset_of('_PyInterpreterFrame', 'previous'),
    'PyInterpreterFrame.Prev_Instr': offset_of('_PyInterpreterFrame', 'prev_instr'),
    'PyInterpreterFrame.Owner': offset_of('_PyInterpreterFrame', 'owner'),

    'PyCFrame.CurrentFrame': offset_of('_PyCFrame', 'current_frame'),
}

for field, value in fields.items():
    if value is None:
        print(f"vms.{field}: <field not present>")
    else:
        print(f"vms.{field}: dec={value} hex=0x{value:x}")
