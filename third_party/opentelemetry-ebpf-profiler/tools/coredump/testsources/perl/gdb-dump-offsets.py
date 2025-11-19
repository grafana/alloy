"""
gdb python script for dumping the Perl introspection data.
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
    return int(gdb.parse_and_eval(f'(unsigned long)(&(({ns} {ty}*)0)->{field})'))

@no_member_to_none
def size_of(ty, *, ns='struct'):
    return int(gdb.parse_and_eval(f'sizeof({ns} {ty})'))


fields = {
    'interpreter.curcop': offset_of('interpreter', 'Icurcop'),
    'interpreter.curstackinfo': offset_of('interpreter', 'Icurstackinfo'),

    'stackinfo.si_cxstack': offset_of('stackinfo', 'si_cxstack'),
    'stackinfo.si_next': offset_of('stackinfo', 'si_next'),
    'stackinfo.si_cxix': offset_of('stackinfo', 'si_cxix'),
    'stackinfo.si_type': offset_of('stackinfo', 'si_type'),

    'context.cx_type': offset_of('context', 'cx_u.cx_blk.blku_type'), # shared header for all types
    'context.blk_oldcop': offset_of('context', 'cx_u.cx_blk.blku_oldcop'),
    'context.blk_sub_retop': offset_of('context', 'cx_u.cx_blk.blk_u.blku_sub.retop'),
    'context.blk_sub_cv': offset_of('context', 'cx_u.cx_blk.blk_u.blku_sub.cv'),
    'context.sizeof': size_of('context'),

    'cop.cop_line': offset_of('cop', 'cop_line'),
    'cop.cop_file': offset_of('cop', 'cop_file'),
    'cop.sizeof': size_of('cop'),

    'sv.sv_any': offset_of('sv', 'sv_any'),
    'sv.sv_flags': offset_of('sv', 'sv_flags'),
    'sv.svu_gp': offset_of('sv', 'sv_u.svu_gp'),
    'sv.svu_hash': offset_of('sv', 'sv_u.svu_hash'),
    'sv.sizeof': size_of('sv'),

    'xpvcv.xcv_flags': offset_of('xpvcv', 'xcv_flags'),
    'xpvcv.xcv_gv': offset_of('xpvcv', 'xcv_gv_u.xcv_gv'),

    'xpvgv.xivu_namehek': offset_of('xpvgv', 'xiv_u.xivu_namehek'),
    'xpvgv.xgv_stash': offset_of('xpvgv', 'xnv_u.xgv_stash'),

    'xpvhv.xhv_max': offset_of('xpvhv', 'xhv_max'),

    'xpvhv_aux.xhv_name_u': offset_of('xpvhv_aux', 'xhv_name_u'),
    'xpvhv_aux.xhv_name_count': offset_of('xpvhv_aux', 'xhv_name_count'),
    'xpvhv_aux.pointer_size': size_of('hek *'),
    'xpvhv_aux.sizeof': size_of('xpvhv_aux'),

    'xpvhv_with_aux.xpvhv_aux': offset_of('xpvhv_with_aux', 'xhv_aux'),
    'xpvhv_with_aux.sizeof': size_of('xpvhv_with_aux'),

    'gp.gp_egv': offset_of('gp', 'gp_egv'),

    'hek.hek_len': offset_of('hek', 'hek_len'),
    'hek.hek_key': offset_of('hek', 'hek_key'),
}


for field, value in fields.items():
    if value is None:
        print(f"vms.{field}: <field not present>")
    else:
        print(f"vms.{field}: dec={value} hex=0x{value:x}")
