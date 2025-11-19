// Code generated from metrics.json. DO NOT EDIT.

package metrics

// To add a new metric append an entry to metrics.json. ONLY APPEND !
// Then run 'make generate' from the top directory.

// Below are the different metric IDs that we currently implement.
const (

	// Leave out the 0 value. It's an indication of not explicitly initialized variables.
	IDInvalid = 0

	// Number of calls to interpreter unwinding in dispatch_interpreters()
	IDUnwindCallInterpreter = 8

	// Unwind attempts since the previous check
	IDUnwindNativeAttempts = 10

	// Unwound frames since the previous check
	IDUnwindNativeFrames = 11

	// Number of times MAX_FRAME_UNWINDS has been exceeded in unwind_next_frame()
	IDUnwindErrStackLengthExceeded = 12

	// Number of failed range searches within 20 steps in get_stack_delta()
	IDUnwindNativeErrLookupTextSection = 13

	// Number of failures to get stack_unwind_info from big_stack_deltas in get_stack_delta()
	IDUnwindNativeErrLookupIterations = 14

	// Number of failures to get stack_unwind_info from big_stack_deltas in get_stack_delta()
	IDUnwindNativeErrLookupRange = 15

	// Number of kernel addresses passed to get_text_section()
	IDUnwindNativeErrKernelAddress = 16

	// Number of failures to find the text section in get_text_section()
	IDUnwindNativeErrWrongTextSection = 17

	// Number of failures due to PC == 0 in unwind_next_frame()
	IDUnwindErrZeroPC = 18

	// Number of attempted python unwinds
	IDUnwindPythonAttempts = 19

	// Number of unwound python frames
	IDUnwindPythonFrames = 20

	// Number of failures to read from pyinfo->pyThreadStateCurrentAddr
	IDUnwindPythonErrBadPyThreadStateCurrentAddr = 21

	// Number of PyThreadState being 0x0
	IDUnwindPythonErrZeroThreadState = 22

	// Number of failures to read from the TLS
	IDUnwindErrBadTLSAddr = 24

	// Number of failures to get the TLS base in tls_get_base()
	IDUnwindErrBadTPBaseAddr = 26

	// Number of failures to read PyThreadState.frame in unwind_python()
	IDUnwindPythonErrBadThreadStateFrameAddr = 27

	// Number of NULL code objects found in process_python_frame()
	IDUnwindPythonZeroFrameCodeObject = 30

	// Number of failures to get code object's argcount in process_python_frame()
	IDUnwindPythonErrBadCodeObjectArgCountAddr = 36

	// The number of executables loaded to eBPF maps
	IDNumExeIDLoadedToEBPF = 43

	// Current size of the hash map pid_page_to_mapping_info
	IDHashmapPidPageToMappingInfo = 44

	// Number of invalid stack deltas in the native unwinder
	IDUnwindNativeErrStackDeltaInvalid = 47

	// Number of times unwind_stop is called without a trace
	IDErrEmptyStack = 48

	// Number of attempted Hotspot frame unwinds
	IDUnwindHotspotAttempts = 50

	// Number of unwound Hotspot frames
	IDUnwindHotspotFrames = 51

	// Number of failures to get hotspot codeblob address (no heap or bad segmap)
	IDUnwindHotspotErrNoCodeblob = 52

	// Number of failures to get codeblob data
	IDUnwindHotspotErrInvalidCodeblob = 53

	// Number of failures to unwind interpreter due to invalid FP
	IDUnwindHotspotErrInterpreterFP = 54

	// Number of successfully symbolized python frames
	IDPythonSymbolizationSuccesses = 55

	// Number of Python frames that failed symbolization
	IDPythonSymbolizationFailures = 56

	// Number of successfully symbolized hotspot frames
	IDHotspotSymbolizationSuccesses = 57

	// Number of Hotspot frames that failed symbolization
	IDHotspotSymbolizationFailures = 58

	// Number of times that PC hold a value smaller than 0x1000
	IDUnwindNativeSmallPC = 59

	// Number of lost perf events in the communication between kernel and user space (report_events)
	IDPerfEventLost = 67

	// Number of stop stack deltas in the native unwinder (success)
	IDUnwindNativeStackDeltaStop = 68

	// Number of times failure to read PC from unwound stack (invalid stack delta)
	IDUnwindNativeErrPCRead = 69

	// Number of times that a lookup of a inner map for stack deltas failed
	IDUnwindNativeErrLookupStackDeltaInnerMap = 70

	// Number of times that a lookup of the outer map for stack deltas failed
	IDUnwindNativeErrLookupStackDeltaOuterMap = 71

	// Number of times the bpf helper failed to get the current comm of the task
	IDErrBPFCurrentComm = 75

	// Number of attempted PHP unwinds
	IDUnwindPHPAttempts = 76

	// Number of unwound PHP frames
	IDUnwindPHPFrames = 77

	// Number of failures to read PHP current execute data pointer
	IDUnwindPHPErrBadCurrentExecuteData = 78

	// Number of failures to read PHP execute data contents
	IDUnwindPHPErrBadZendExecuteData = 79

	// Number of failures to read PHP zend function contents
	IDUnwindPHPErrBadZendFunction = 80

	// Number of failures to read PHP zend opline contents
	IDUnwindPHPErrBadZendOpline = 81

	// Number of LRU hits for kernel symbols
	IDKernelFallbackSymbolLRUHit = 82

	// Number of LRU mises for kernel symbols
	IDKernelFallbackSymbolLRUMiss = 83

	// Number of cache hits for ELF information
	IDELFInfoCacheHit = 84

	// Number of cache misses for ELF information
	IDELFInfoCacheMiss = 85

	// Number of successfully symbolized PHP frames
	IDPHPSymbolizationSuccess = 86

	// Number of PHP frames that failed symbolization
	IDPHPSymbolizationFailure = 87

	// Number of cache hits for Python AddrToCodeObject
	IDPythonAddrToCodeObjectHit = 88

	// Number of cache misses for Python AddrToCodeObject
	IDPythonAddrToCodeObjectMiss = 89

	// Number of cache hits for Hotspot AddrToSymbol
	IDHotspotAddrToSymbolHit = 90

	// Number of cache misses for Hotspot AddrToSymbol
	IDHotspotAddrToSymbolMiss = 91

	// Number of cache hits for Hotspot AddrToMethod
	IDHotspotAddrToMethodHit = 92

	// Number of cache misses for Hotspot AddrToMethod
	IDHotspotAddrToMethodMiss = 93

	// Number of cache hits for Hotspot AddrToJITInfo
	IDHotspotAddrToJITInfoHit = 94

	// Number of cache misses for Hotspot AddrToJITInfo
	IDHotspotAddrToJITInfoMiss = 95

	// Number of cache hits for PHP AddrToFunc
	IDPHPAddrToFuncHit = 96

	// Number of cache misses for PHP AddrToFunc
	IDPHPAddrToFuncMiss = 97

	// Number of times a perf event was received without data (report_events)
	IDPerfEventNoData = 101

	// Number of times a perf event read failed (report_events)
	IDPerfEventReadError = 102

	// Number of successfully symbolized Ruby frames
	IDRubySymbolizationSuccess = 106

	// Number of Ruby frames that failed symbolization
	IDRubySymbolizationFailure = 107

	// Number of attempted Ruby unwinds
	IDUnwindRubyAttempts = 108

	// Number of unwound Ruby frames
	IDUnwindRubyFrames = 109

	// Number of cache hits for Ruby AddrToString
	IDRubyAddrToStringHit = 112

	// Number of cache misses for Ruby AddrToString
	IDRubyAddrToStringMiss = 113

	// Number of attempted perl unwinds
	IDUnwindPerlAttempts = 115

	// Number of unwound perl frames
	IDUnwindPerlFrames = 116

	// Number of failures to read perl TLS info
	IDUnwindPerlTLS = 117

	// Number of failures to read perl stack info
	IDUnwindPerlReadStackInfo = 118

	// Number of failures to read perl context stack entry
	IDUnwindPerlReadContextStackEntry = 119

	// Number of failures to resolve perl EGV
	IDUnwindPerlResolveEGV = 120

	// Number of successfully symbolized Perl frames
	IDPerlSymbolizationSuccess = 121

	// Number of Perl frames that failed symbolization
	IDPerlSymbolizationFailure = 122

	// Number of cache hits for Perl AddrToHEK
	IDPerlAddrToHEKHit = 123

	// Number of cache misses for Perl AddrToHEK
	IDPerlAddrToHEKMiss = 124

	// Number of cache hits for Perl AddrToCOP
	IDPerlAddrToCOPHit = 125

	// Number of cache misses for Perl AddrToCOP
	IDPerlAddrToCOPMiss = 126

	// Number of cache hits for Perl AddrToGV
	IDPerlAddrToGVHit = 127

	// Number of cache misses for Perl AddrToGV
	IDPerlAddrToGVMiss = 128

	// Number of failures to unwind because return address was not found with heuristic
	IDUnwindHotspotErrInvalidRA = 130

	// Current size of the unwind info array
	IDUnwindInfoArraySize = 133

	// Current size of the stack delta pages hash map
	IDHashmapNumStackDeltaPages = 134

	// Number of attempted V8 unwinds
	IDUnwindV8Attempts = 136

	// Number of unwound V8 frames
	IDUnwindV8Frames = 137

	// Number of failures to read V8 frame pointer data
	IDUnwindV8ErrBadFP = 138

	// Number of failures to read V8 Code/JSFunction object
	IDUnwindV8ErrBadJSFunc = 139

	// Number of failures to read V8 Code object
	IDUnwindV8ErrBadCode = 140

	// Number of successfully symbolized V8 frames
	IDV8SymbolizationSuccess = 141

	// Number of V8 frames that failed symbolization
	IDV8SymbolizationFailure = 142

	// Number of cache hits for V8 strings
	IDV8AddrToStringHit = 143

	// Number of cache misses for V8 strings
	IDV8AddrToStringMiss = 144

	// Number of cache hits for V8 SharedFunctionInfo
	IDV8AddrToSFIHit = 145

	// Number of cache misses for V8 SharedFunctionInfo
	IDV8AddrToSFIMiss = 146

	// Number of cache hits for V8 Code/JSFunction
	IDV8AddrToFuncHit = 147

	// Number of cache misses for V8 Code/JSFunction
	IDV8AddrToFuncMiss = 148

	// Number of cache hits for V8 Source
	IDV8AddrToSourceHit = 149

	// Number of cache misses for V8 Source
	IDV8AddrToSourceMiss = 150

	// Number of cache hits for Hotspot AddrToStubNameID
	IDHotspotAddrToStubNameIDHit = 151

	// Number of cache misses for Hotspot AddrToStubNameID
	IDHotspotAddrToStubNameIDMiss = 152

	// Number of times reading /proc/<PID> as it does not exist anymore
	IDErrProcNotExist = 156

	// Number of times process exits while reading /proc/<PID>
	IDErrProcESRCH = 157

	// Number of times reading /proc/<PID> failed due to missing permission
	IDErrProcPerm = 158

	// Number of added cache elements for Perl AddrToHEK
	IDPerlAddrToHEKAdd = 161

	// Number of deleted cache elements for Perl AddrToHEK
	IDPerlAddrToHEKDel = 162

	// Number of added cache elements for Perl AddrToCOP
	IDPerlAddrToCOPAdd = 163

	// Number of deleted cache elements for Perl AddrToCOP
	IDPerlAddrToCOPDel = 164

	// Number of added cache elements for Perl AddrToGV
	IDPerlAddrToGVAdd = 165

	// Number of deleted cache elementes Perl AddrToGV
	IDPerlAddrToGVDel = 166

	// Number of added cache elements for Hotspot AddrToSymbol
	IDHotspotAddrToSymbolAdd = 167

	// Number of deleted cache elements for Hotspot AddrToSymbol
	IDHotspotAddrToSymbolDel = 168

	// Number of added cache elements for Hotspot AddrToMethod
	IDHotspotAddrToMethodAdd = 169

	// Number of deleted cache elements for Hotspot AddrToMethod
	IDHotspotAddrToMethodDel = 170

	// Number of added cache elements for Hotspot AddrToJITInfo
	IDHotspotAddrToJITInfoAdd = 171

	// Number of deleted cache elements for Hotspot AddrToJITInfo
	IDHotspotAddrToJITInfoDel = 172

	// Number of added cache elements for Hotspot AddrToStubNameID
	IDHotspotAddrToStubNameIDAdd = 173

	// Number of deleted cache elements for Hotspot AddrToStubNameID
	IDHotspotAddrToStubNameIDDel = 174

	// Number of added cache elements for PHP AddrToFunc
	IDPHPAddrToFuncAdd = 175

	// Number of deleted cache elements for PHP AddrToFunc
	IDPHPAddrToFuncDel = 176

	// Number of added cache elements for Python AddrToCodeObject
	IDPythonAddrToCodeObjectAdd = 177

	// Number of deleted cache elements for Python AddrToCodeObject
	IDPythonAddrToCodeObjectDel = 178

	// Number of added cache elements for Ruby AddrToString
	IDRubyAddrToStringAdd = 181

	// Number of deleted cache elements for Ruby AddrToString
	IDRubyAddrToStringDel = 182

	// Number of added cache elements for V8 strings
	IDV8AddrToStringAdd = 183

	// Number of deleted cache elements for V8 strings
	IDV8AddrToStringDel = 184

	// Number of added cache elements for V8 SharedFunctionInfo
	IDV8AddrToSFIAdd = 185

	// Number of deleted cache elements for V8 SharedFunctionInfo
	IDV8AddrToSFIDel = 186

	// Number of added cache elements for V8 Code/JSFunction
	IDV8AddrToFuncAdd = 187

	// Number of deleted cache elements for V8 Code/JSFunction
	IDV8AddrToFuncDel = 188

	// Number of added cache elements for V8 Source
	IDV8AddrToSourceAdd = 189

	// Number of deleted cache elements for V8 Source
	IDV8AddrToSourceDel = 190

	// Number of times we failed to update reported_pids
	IDReportedPIDsErr = 191

	// Maximum number of size that was requested within the last reporting interval
	IDRubyMaxSize = 192

	// Maximum number of hekLen that was requested within the last reporting interval
	IDPerlHekLen = 193

	// Number of times frame unwinding failed because of LR == 0
	IDUnwindNativeLr0 = 194

	// Number of times updating an element in unwindInfoArray failed
	IDUnwindInfoArrayUpdate = 195

	// Number of times updating an element in exeIDToStackDeltas failed
	IDExeIDToStackDeltasUpdate = 196

	// Number of times updating an element in stackDeltaPageToInfo failed
	IDStackDeltaPageToInfoUpdate = 198

	// Number of times deleting an element from stackDeltaPageToInfo failed
	IDStackDeltaPageToInfoDelete = 199

	// Number of times updating an element in pidPageToMappingInfo failed
	IDPidPageToMappingInfoUpdate = 200

	// Number of times deleting an element from pidPageToMappingInfo failed
	IDPidPageToMappingInfoDelete = 201

	// Number of times the stack delta provider failed to extract stack deltas
	IDStackDeltaProviderExtractionError = 204

	// Number of cache hits in tracehandler trace cache by UM hash
	IDTraceCacheHit = 205

	// Number of cache misses in tracehandler trace cache by UM hash
	IDTraceCacheMiss = 206

	// Number of /proc/PID/maps process attempts
	IDNumProcAttempts = 207

	// Number of times finding the return address in the interpreter loop failed for PHP 8+.
	IDPHPFailedToFindReturnAddress = 208

	// Number of times we encountered frame sizes larger than the supported maximum
	IDHotspotUnsupportedFrameSize = 209

	// Number of new PID events (report_events)
	IDNumProcNew = 213

	// Number of exit PID events (report_events)
	IDNumProcExit = 214

	// Number of unknown PC events (report_events)
	IDNumUnknownPC = 215

	// Max /proc/PID/maps parse time for a single collection interval, in microseconds
	IDMaxProcParseUsec = 218

	// Time spent processing /proc/PID/maps on startup, in milliseconds
	IDProcPIDStartupMs = 219

	// Total /proc/PID/maps parse time for a single collection interval, in microseconds
	IDTotalProcParseUsec = 220

	// Number of generic PID events (report_events)
	IDNumGenericPID = 226

	// Number of times we failed to update pid_events
	IDPIDEventsErr = 227

	// Number of failures to read _PyCFrame.current_frame in unwind_python()
	IDUnwindPythonErrBadCFrameFrameAddr = 228

	// Number of times stack unwinding was stopped to not hit the limit of tail calls
	IDMaxTailCalls = 229

	// Indicates if probabilistic profiling is enabled or disabled: 1 profiling is enabled, -1 profiling is disabled.
	IDProbProfilingStatus = 230

	// Interval in seconds for which probabilistic profiling will be enabled or disabled.
	IDProbProfilingInterval = 231

	// Number of times enabling a perf event hook failed
	IDPerfEventEnableErr = 232

	// Number of times disabling a perf event hook failed
	IDPerfEventDisableErr = 233

	// Number of times we didn't find an entry for this process in the Python process info array
	IDUnwindPythonErrNoProcInfo = 234

	// Number of failures to read autoTLSkey
	IDUnwindPythonErrBadAutoTlsKeyAddr = 235

	// Number of failures to read the thread state pointer from TLD
	IDUnwindPythonErrReadThreadStateAddr = 236

	// Number of failures to determine the base address for thread-specific data
	IDUnwindPythonErrReadTsdBase = 237

	// Number of times we didn't find an entry for this process in the Ruby process info array
	IDUnwindRubyErrNoProcInfo = 238

	// Number of failures to read the stack pointer from the Ruby context
	IDUnwindRubyErrReadStackPtr = 239

	// Number of failures to read the size of the VM stack from the Ruby context
	IDUnwindRubyErrReadStackSize = 240

	// Number of failures to read the control frame pointer from the Ruby context
	IDUnwindRubyErrReadCfp = 241

	// Number of failures to read the expression path from the Ruby frame
	IDUnwindRubyErrReadEp = 242

	// Number of failures to read the instruction sequence body
	IDUnwindRubyErrReadIseqBody = 243

	// Number of failures to read the instruction sequence encoded size
	IDUnwindRubyErrReadIseqEncoded = 244

	// Number of failures to read the instruction sequence size
	IDUnwindRubyErrReadIseqSize = 245

	// Number of times the unwind instructions requested LR unwinding mid-trace
	IDUnwindNativeErrLrUnwindingMidTrace = 246

	// Number of failures to read the kernel-mode registers
	IDUnwindNativeErrReadKernelModeRegs = 247

	// Number of failures to read the IRQ stack link
	IDUnwindNativeErrChaseIrqStackLink = 248

	// Number of times we didn't find an entry for this process in the V8 process info array
	IDUnwindV8ErrNoProcInfo = 249

	// Number of times an unwind_info_array index was invalid
	IDUnwindNativeErrBadUnwindInfoIndex = 250

	// Number of times batch updating elements in exeIDToStackDeltas failed
	IDExeIDToStackDeltasBatchUpdate = 251

	// Number of times batch updating elements in stackDeltaPageToInfo failed
	IDStackDeltaPageToInfoBatchUpdate = 252

	// Number of times batch deleting elements from pidPageToMappingInfo failed
	IDPidPageToMappingInfoBatchDelete = 253

	// Number of times the Hotspot unwind instructions requested LR unwinding mid-trace
	IDUnwindHotspotErrLrUnwindingMidTrace = 256

	// Number of failures to get TSD base for APM correlation
	IDUnwindApmIntErrReadTsdBase = 257

	// Number of failures read the APM correlation pointer
	IDUnwindApmIntErrReadCorrBufPtr = 258

	// Number of failures read the APM correlation buffer
	IDUnwindApmIntErrReadCorrBuf = 259

	// Number of successful reads of APM correlation info
	IDUnwindApmIntReadSuccesses = 260

	// Number of attempted dotnet unwinds
	IDUnwindDotnetAttempts = 261

	// Number of unwound dotnet frames
	IDUnwindDotnetFrames = 262

	// Number of times we didn't find an entry for this process in the dotnet process info array
	IDUnwindDotnetErrNoProcInfo = 263

	// Number of failures to read dotnet frame pointer data
	IDUnwindDotnetErrBadFP = 264

	// Number of failures to read dotnet CodeHeader
	IDUnwindDotnetErrCodeHeader = 265

	// Number of failures to unwind dotnet frame due to large code size
	IDUnwindDotnetErrCodeTooLarge = 266

	// Number of successfully symbolized dotnet frames
	IDDotnetSymbolizationSuccesses = 267

	// Number of dotnet frames that failed symbolization
	IDDotnetSymbolizationFailures = 268

	// Number of cache hits for dotnet AddrToMethod
	IDDotnetAddrToMethodHit = 269

	// Number of cache misses for dotnet AddrToMethod
	IDDotnetAddrToMethodMiss = 270

	// Number of times the stack delta provider succeeded to extract stack deltas
	IDStackDeltaProviderSuccess = 271

	// Number of lost trace events in the communication between kernel and user space (trace_events)
	IDTraceEventLost = 272

	// Number of times a trace event was received without data (trace_events)
	IDTraceEventNoData = 273

	// Number of times a trace event read failed (trace_events)
	IDTraceEventReadError = 274

	// Number of parsing errors seen during processing /proc/<PID>/maps
	IDErrProcParse = 275

	// Number of successfully symbolized Go frames
	IDGoSymbolizationSuccess = 276

	// Number of Go frames that failed symbolization
	IDGoSymbolizationFailure = 277

	// Number of attempts to read Go custom labels
	IDUnwindGoLabelsAttempts = 278

	// Number of failures reading Go custom labels
	IDUnwindGoLabelsFailures = 279

	// max number of ID values, keep this as *last entry*
	IDMax = 280
)
