"""
gdb python script for dumping the Ruby vmStruct offsets.
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
    return int(gdb.parse_and_eval(f'(uintptr_t)(&((struct {ty}*)0)->{field})'))

@no_member_to_none
def size_of(ty, *, ns='struct'):
    return int(gdb.parse_and_eval(f'sizeof({ns} {ty})'))

@no_member_to_none
def size_of_field(ty, field):
    return int(gdb.parse_and_eval(f'sizeof(((struct {ty}*)0)->{field})'))


fields = {
    'execution_context_struct.vm_stack': offset_of('rb_execution_context_struct', 'vm_stack'),
    'execution_context_struct.vm_stack_size': offset_of('rb_execution_context_struct', 'vm_stack_size'),
    'execution_context_struct.cfp': offset_of('rb_execution_context_struct', 'cfp'),

    'control_frame_struct.pc': offset_of('rb_control_frame_struct', 'pc'),
    'control_frame_struct.iseq': offset_of('rb_control_frame_struct', 'iseq'),
    'control_frame_struct.ep': offset_of('rb_control_frame_struct', 'ep'),
    'control_frame_struct.size_of_control_frame_struct': size_of('rb_control_frame_struct'),

    'iseq_struct.body': offset_of('rb_iseq_struct', 'body'),

    'iseq_constant_body.iseq_type': offset_of('rb_iseq_constant_body', 'type'),
    'iseq_constant_body.size': offset_of('rb_iseq_constant_body', 'iseq_size'),
    'iseq_constant_body.encoded': offset_of('rb_iseq_constant_body', 'iseq_encoded'),
    'iseq_constant_body.location': offset_of('rb_iseq_constant_body', 'location'),
    'iseq_constant_body.insn_info_body': offset_of('rb_iseq_constant_body', 'insns_info.body'),
    'iseq_constant_body.insn_info_size': offset_of('rb_iseq_constant_body', 'insns_info.size'),
    'iseq_constant_body.succ_index_table': offset_of('rb_iseq_constant_body', 'insns_info.succ_index_table'),
    'iseq_constant_body.size_of_iseq_constant_body': size_of('rb_iseq_constant_body'),

    'iseq_location_struct.pathobj': offset_of('rb_iseq_location_struct', 'pathobj'),
    'iseq_location_struct.base_label': offset_of('rb_iseq_location_struct', 'base_label'),

    'iseq_insn_info_entry.position': offset_of('iseq_insn_info_entry', 'position'),
    'iseq_insn_info_entry.size_of_position': size_of_field('iseq_insn_info_entry', 'position'),
    'iseq_insn_info_entry.line_no': offset_of('iseq_insn_info_entry', 'line_no'),
    'iseq_insn_info_entry.size_of_line_no': size_of_field('iseq_insn_info_entry', 'line_no'),
    'iseq_insn_info_entry.size_of_iseq_insn_info_entry': size_of('iseq_insn_info_entry'),

    'rstring_struct.as_ary': offset_of('RString', 'as.embed.ary'),
    'rstring_struct.as_heap_ptr': offset_of('RString', 'as.heap.ptr'),

    'rarray_struct.as_ary': offset_of('RArray', 'as.ary'),
    'rarray_struct.as_heap_ptr': offset_of('RArray', 'as.heap.ptr'),

    'succ_index_table_struct.small_block_ranks': offset_of('succ_dict_block', 'small_block_ranks'),
    'succ_index_table_struct.block_bits': offset_of('succ_dict_block', 'bits'),
    'succ_index_table_struct.succ_part': offset_of('succ_index_table', 'succ_part'),
    'succ_index_table_struct.size_of_succ_dict_block': size_of('succ_dict_block'),
    'size_of_immediate_table': size_of_field('succ_index_table', 'imm_part') * 9 // 8,

    'size_of_value': size_of('VALUE', ns=''),

    'rb_ractor_struct.running_ec': offset_of('rb_ractor_struct', 'threads.running_ec'),
}


for field, value in fields.items():
    if value is None:
        print(f"vms.{field}: <field not present>")
    else:
        print(f"vms.{field}: dec={value} hex=0x{value:x}")
