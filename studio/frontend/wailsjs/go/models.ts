export namespace config {
	
	export class Input {
	    encoding?: string;
	    bom?: string;
	    delimiter?: string;
	    quote?: string;
	    escape?: string;
	    header?: boolean;
	    header_row?: number;
	    skip_rows_after_header?: number;
	    skip_footer?: number;
	    comment_prefix?: string;
	    null_tokens?: string[];
	    trim?: boolean;
	    ragged_rows?: string;
	    duplicate_headers?: string;
	
	    static createFrom(source: any = {}) {
	        return new Input(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.encoding = source["encoding"];
	        this.bom = source["bom"];
	        this.delimiter = source["delimiter"];
	        this.quote = source["quote"];
	        this.escape = source["escape"];
	        this.header = source["header"];
	        this.header_row = source["header_row"];
	        this.skip_rows_after_header = source["skip_rows_after_header"];
	        this.skip_footer = source["skip_footer"];
	        this.comment_prefix = source["comment_prefix"];
	        this.null_tokens = source["null_tokens"];
	        this.trim = source["trim"];
	        this.ragged_rows = source["ragged_rows"];
	        this.duplicate_headers = source["duplicate_headers"];
	    }
	}
	export class Logging {
	    level?: string;
	    format?: string;
	    file?: string;
	    summary?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Logging(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.level = source["level"];
	        this.format = source["format"];
	        this.file = source["file"];
	        this.summary = source["summary"];
	    }
	}
	export class Pair {
	    from: string;
	    to: string;
	
	    static createFrom(source: any = {}) {
	        return new Pair(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.from = source["from"];
	        this.to = source["to"];
	    }
	}
	export class Rule {
	    source?: string;
	    source_index?: number;
	    optional?: boolean;
	    if_missing?: string;
	    if_empty?: string;
	    default?: string;
	    on_error?: string;
	    map?: Pair[];
	    map_unmatched?: string;
	    trim?: boolean;
	    formats?: string[];
	    decimal_sep?: string;
	    thousands_sep?: string;
	    true_values?: string[];
	    false_values?: string[];
	    timezone?: string;
	
	    static createFrom(source: any = {}) {
	        return new Rule(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.source = source["source"];
	        this.source_index = source["source_index"];
	        this.optional = source["optional"];
	        this.if_missing = source["if_missing"];
	        this.if_empty = source["if_empty"];
	        this.default = source["default"];
	        this.on_error = source["on_error"];
	        this.map = this.convertValues(source["map"], Pair);
	        this.map_unmatched = source["map_unmatched"];
	        this.trim = source["trim"];
	        this.formats = source["formats"];
	        this.decimal_sep = source["decimal_sep"];
	        this.thousands_sep = source["thousands_sep"];
	        this.true_values = source["true_values"];
	        this.false_values = source["false_values"];
	        this.timezone = source["timezone"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class OnError {
	    strategy?: string;
	    max_error_rate?: number;
	    max_errors?: number;
	    reject_file?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new OnError(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.strategy = source["strategy"];
	        this.max_error_rate = source["max_error_rate"];
	        this.max_errors = source["max_errors"];
	        this.reject_file = source["reject_file"];
	    }
	}
	export class TargetColumn {
	    name: string;
	    type: string;
	    required?: boolean;
	    default?: string;
	    max_length?: number;
	    on_overflow?: string;
	    case?: string;
	    pattern?: string;
	    allowed_values?: string[];
	    min?: string;
	    max?: string;
	    scale?: number;
	    rounding?: string;
	    decimal_sep?: string;
	    thousands_sep?: string;
	    true_value?: string;
	    false_value?: string;
	    format?: string;
	    timezone?: string;
	
	    static createFrom(source: any = {}) {
	        return new TargetColumn(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.type = source["type"];
	        this.required = source["required"];
	        this.default = source["default"];
	        this.max_length = source["max_length"];
	        this.on_overflow = source["on_overflow"];
	        this.case = source["case"];
	        this.pattern = source["pattern"];
	        this.allowed_values = source["allowed_values"];
	        this.min = source["min"];
	        this.max = source["max"];
	        this.scale = source["scale"];
	        this.rounding = source["rounding"];
	        this.decimal_sep = source["decimal_sep"];
	        this.thousands_sep = source["thousands_sep"];
	        this.true_value = source["true_value"];
	        this.false_value = source["false_value"];
	        this.format = source["format"];
	        this.timezone = source["timezone"];
	    }
	}
	export class Output {
	    encoding?: string;
	    bom?: boolean;
	    delimiter?: string;
	    quote?: string;
	    quote_style?: string;
	    escape?: string;
	    line_ending?: string;
	    header?: boolean;
	    null_value?: string;
	
	    static createFrom(source: any = {}) {
	        return new Output(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.encoding = source["encoding"];
	        this.bom = source["bom"];
	        this.delimiter = source["delimiter"];
	        this.quote = source["quote"];
	        this.quote_style = source["quote_style"];
	        this.escape = source["escape"];
	        this.line_ending = source["line_ending"];
	        this.header = source["header"];
	        this.null_value = source["null_value"];
	    }
	}
	export class Target {
	    version?: number;
	    name?: string;
	    description?: string;
	    output: Output;
	    columns: TargetColumn[];
	
	    static createFrom(source: any = {}) {
	        return new Target(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.version = source["version"];
	        this.name = source["name"];
	        this.description = source["description"];
	        this.output = this.convertValues(source["output"], Output);
	        this.columns = this.convertValues(source["columns"], TargetColumn);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class TargetRef {
	    path?: string;
	    inline?: Target;
	
	    static createFrom(source: any = {}) {
	        return new TargetRef(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.inline = this.convertValues(source["inline"], Target);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class Mapping {
	    version: number;
	    name: string;
	    description?: string;
	    target: TargetRef;
	    input: Input;
	    on_error: OnError;
	    logging: Logging;
	    columns: Record<string, Rule>;
	
	    static createFrom(source: any = {}) {
	        return new Mapping(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.version = source["version"];
	        this.name = source["name"];
	        this.description = source["description"];
	        this.target = this.convertValues(source["target"], TargetRef);
	        this.input = this.convertValues(source["input"], Input);
	        this.on_error = this.convertValues(source["on_error"], OnError);
	        this.logging = this.convertValues(source["logging"], Logging);
	        this.columns = this.convertValues(source["columns"], Rule, true);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	
	
	
	
	

}

export namespace detect {
	
	export class Column {
	    index: number;
	    name: string;
	    target: string;
	    type: string;
	    confidence: number;
	    values: number;
	    empty: number;
	    formats?: string[];
	    decimal_sep?: string;
	    thousands_sep?: string;
	    true_values?: string[];
	    false_values?: string[];
	    scale?: number;
	    near_type?: string;
	    near_confidence?: number;
	    near_formats?: string[];
	    near_decimal_sep?: string;
	    near_thousands_sep?: string;
	    max_length: number;
	    null_tokens?: string[];
	    examples?: string[];
	    failing?: string[];
	    notes?: string[];
	
	    static createFrom(source: any = {}) {
	        return new Column(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.index = source["index"];
	        this.name = source["name"];
	        this.target = source["target"];
	        this.type = source["type"];
	        this.confidence = source["confidence"];
	        this.values = source["values"];
	        this.empty = source["empty"];
	        this.formats = source["formats"];
	        this.decimal_sep = source["decimal_sep"];
	        this.thousands_sep = source["thousands_sep"];
	        this.true_values = source["true_values"];
	        this.false_values = source["false_values"];
	        this.scale = source["scale"];
	        this.near_type = source["near_type"];
	        this.near_confidence = source["near_confidence"];
	        this.near_formats = source["near_formats"];
	        this.near_decimal_sep = source["near_decimal_sep"];
	        this.near_thousands_sep = source["near_thousands_sep"];
	        this.max_length = source["max_length"];
	        this.null_tokens = source["null_tokens"];
	        this.examples = source["examples"];
	        this.failing = source["failing"];
	        this.notes = source["notes"];
	    }
	}
	export class Result {
	    encoding: string;
	    bom: boolean;
	    delimiter: string;
	    quote: string;
	    escape: string;
	    header: boolean;
	    header_row: number;
	    skip_footer: number;
	    trim: boolean;
	    null_tokens?: string[];
	    ragged_rows: string;
	    duplicate_headers: boolean;
	    columns: Column[];
	    sample_rows: number;
	    whole_file: boolean;
	    notes?: string[];
	
	    static createFrom(source: any = {}) {
	        return new Result(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.encoding = source["encoding"];
	        this.bom = source["bom"];
	        this.delimiter = source["delimiter"];
	        this.quote = source["quote"];
	        this.escape = source["escape"];
	        this.header = source["header"];
	        this.header_row = source["header_row"];
	        this.skip_footer = source["skip_footer"];
	        this.trim = source["trim"];
	        this.null_tokens = source["null_tokens"];
	        this.ragged_rows = source["ragged_rows"];
	        this.duplicate_headers = source["duplicate_headers"];
	        this.columns = this.convertValues(source["columns"], Column);
	        this.sample_rows = source["sample_rows"];
	        this.whole_file = source["whole_file"];
	        this.notes = source["notes"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace report {
	
	export class ColumnStats {
	    name: string;
	    errors: number;
	    defaulted: number;
	    warnings: number;
	
	    static createFrom(source: any = {}) {
	        return new ColumnStats(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.errors = source["errors"];
	        this.defaulted = source["defaulted"];
	        this.warnings = source["warnings"];
	    }
	}
	export class MissingColumn {
	    source: string;
	    target: string;
	    action: string;
	
	    static createFrom(source: any = {}) {
	        return new MissingColumn(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.source = source["source"];
	        this.target = source["target"];
	        this.action = source["action"];
	    }
	}
	export class Summary {
	    mapping: string;
	    mapping_sha256: string;
	    engine_version: string;
	    status: string;
	    failure_reason?: string;
	    rows_read: number;
	    rows_written: number;
	    rows_rejected: number;
	    rows_with_errors: number;
	    columns?: ColumnStats[];
	    missing_optional_columns?: MissingColumn[];
	    duration_seconds: number;
	
	    static createFrom(source: any = {}) {
	        return new Summary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.mapping = source["mapping"];
	        this.mapping_sha256 = source["mapping_sha256"];
	        this.engine_version = source["engine_version"];
	        this.status = source["status"];
	        this.failure_reason = source["failure_reason"];
	        this.rows_read = source["rows_read"];
	        this.rows_written = source["rows_written"];
	        this.rows_rejected = source["rows_rejected"];
	        this.rows_with_errors = source["rows_with_errors"];
	        this.columns = this.convertValues(source["columns"], ColumnStats);
	        this.missing_optional_columns = this.convertValues(source["missing_optional_columns"], MissingColumn);
	        this.duration_seconds = source["duration_seconds"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

export namespace studio {
	
	export class Event {
	    level: string;
	    line?: number;
	    column?: string;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new Event(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.level = source["level"];
	        this.line = source["line"];
	        this.column = source["column"];
	        this.message = source["message"];
	    }
	}
	export class Issue {
	    severity: string;
	    path: string;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new Issue(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.severity = source["severity"];
	        this.path = source["path"];
	        this.message = source["message"];
	    }
	}
	export class Loaded {
	    mapping?: config.Mapping;
	    target_path?: string;
	
	    static createFrom(source: any = {}) {
	        return new Loaded(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.mapping = this.convertValues(source["mapping"], config.Mapping);
	        this.target_path = source["target_path"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class Opened {
	    path: string;
	    detection?: detect.Result;
	    mapping?: config.Mapping;
	
	    static createFrom(source: any = {}) {
	        return new Opened(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.detection = this.convertValues(source["detection"], detect.Result);
	        this.mapping = this.convertValues(source["mapping"], config.Mapping);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class Platform {
	    os: string;
	    arch: string;
	    label: string;
	    available: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Platform(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.os = source["os"];
	        this.arch = source["arch"];
	        this.label = source["label"];
	        this.available = source["available"];
	    }
	}
	export class RawRow {
	    line: number;
	    kind: string;
	    fields: string[];
	
	    static createFrom(source: any = {}) {
	        return new RawRow(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.line = source["line"];
	        this.kind = source["kind"];
	        this.fields = source["fields"];
	    }
	}
	export class Raw {
	    rows: RawRow[];
	    width: number;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new Raw(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.rows = this.convertValues(source["rows"], RawRow);
	        this.width = source["width"];
	        this.error = source["error"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class Recent {
	    kind: string;
	    path: string;
	    // Go type: time
	    opened: any;
	
	    static createFrom(source: any = {}) {
	        return new Recent(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.kind = source["kind"];
	        this.path = source["path"];
	        this.opened = this.convertValues(source["opened"], null);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class Result {
	    issues?: Issue[];
	    header?: string[];
	    rows?: string[][];
	    rejected?: string[][];
	    events?: Event[];
	    summary?: report.Summary;
	    failure?: string;
	    rejects?: string;
	
	    static createFrom(source: any = {}) {
	        return new Result(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.issues = this.convertValues(source["issues"], Issue);
	        this.header = source["header"];
	        this.rows = source["rows"];
	        this.rejected = source["rejected"];
	        this.events = this.convertValues(source["events"], Event);
	        this.summary = this.convertValues(source["summary"], report.Summary);
	        this.failure = source["failure"];
	        this.rejects = source["rejects"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class TargetDraft {
	    target?: config.Target;
	    detection?: detect.Result;
	
	    static createFrom(source: any = {}) {
	        return new TargetDraft(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.target = this.convertValues(source["target"], config.Target);
	        this.detection = this.convertValues(source["detection"], detect.Result);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}

}

