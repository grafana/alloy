// Copyright The OpenTelemetry Authors
// SPDX-License-Identifier: Apache-2.0

#include <assert.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <stdbool.h>
#include <inttypes.h>
#include "symblib.h"

// Example visitor callback for processing return pads
SymblibStatus retpad_visitor(void* user_data, const SymblibReturnPad* ret_pad) {
    printf("\nReturn pad at ELF VA: 0x%08" PRIx64 "\n", ret_pad->elf_va);

    // Iterate over each entry in the SymblibReturnPad
    for (size_t i = 0; i < ret_pad->entries.len; ++i) {
        SymblibReturnPadEntry* entry = &((SymblibReturnPadEntry*)ret_pad->entries.data)[i];
        printf("\tEntry %zu:\n", i);
        printf("\t\tFunction: %s\n", entry->func ? entry->func : "(null)");
        printf("\t\tFile: %s\n", entry->file ? entry->file : "(null)");
        printf("\t\tLine: %u\n", entry->line);
    }

    return 0;
}

// Example visitor callback for processing ranges
SymblibStatus range_visitor(void* user_data, const SymblibRange* range) {
    printf("\nSymbol range at ELF VA: 0x08%" PRIx64 "\n", range->elf_va);
    printf("\tFunction: %s\n", range->func);
    printf("\tFile: %s\n", range->file ? range->file : "(null)");
    printf("\tCall File: %s\n", range->call_file ? range->call_file : "(null)");
    printf("\tCall Line: %u\n", range->call_line);
    printf("\tDepth: %u\n", range->depth);
    printf("\tLine Table Length: %zu\n", range->line_table.len);

    // Submit the range to the return pad extractor.
    SymblibStatus err = symblib_retpadextr_submit(
        (SymblibRetPadExtractor*)user_data, range, retpad_visitor, NULL);
    if (err != SYMBLIB_OK) {
        fprintf(stderr, "Failed to submit range for extraction\n");
        return err;
    }

    return 0;
}

int main(int argc, const char** argv) {
    const char* executable;

    switch (argc) {
    case 0:
        return EXIT_FAILURE;
    case 1:
        // Use this binary.
        executable = argv[0];
        break;
    default:
        // Use user-passed file.
        executable = argv[1];
    }

    printf("Starting range extraction for executable: %s\n", executable);

    // Initialize the global return pad extractor.
    // We use it in the range extractor visitor.
    SymblibRetPadExtractor* extr = NULL;
    SymblibStatus err = symblib_retpadextr_new(executable, &extr);
    if (err != SYMBLIB_OK) {
        fprintf(stderr, "Failed to create global SymblibRetPadExtractor\n");
        return EXIT_FAILURE;
    }
    assert(extr != NULL);

    // Call the range extraction function with our visitor.
    err = symblib_rangeextr(executable, false, range_visitor, extr);
    if (err != SYMBLIB_OK) {
        fprintf(stderr, "Error during range extraction: %d\n", err);
        symblib_retpadextr_free(extr);
        return EXIT_FAILURE;
    }

    // Notify the return pad extractor that we're done.
    err = symblib_retpadextr_submit(extr, NULL, retpad_visitor, NULL);
    if (err != SYMBLIB_OK) {
        fprintf(stderr, "Failed to submit end-of-ranges marker\n");
        symblib_retpadextr_free(extr);
        return err;
    }

    printf("\nRange extraction completed successfully.\n");

    symblib_retpadextr_free(extr);
    return EXIT_SUCCESS;
}
