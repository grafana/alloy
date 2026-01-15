import { faLink } from '@fortawesome/free-solid-svg-icons';
import { FontAwesomeIcon } from '@fortawesome/react-fontawesome';
import { useCallback, useEffect, useState } from 'react';

import { alloyStringify } from '../alloy-syntax-js/stringify';
import { type Value, ValueType } from '../alloy-syntax-js/types';

interface AsyncStringifiedValueProps {
  value: Value;
  maxLength?: number;
}

type LargeValueState =
  | { status: 'loading' } // Shows greyed-out download text
  | { status: 'ready'; result: string }; // Shows clickable download link

/**
 * Estimates the size of a value to determine if it will likely exceed maxLength.
 * Returns a rough count of elements (array length, object keys, string length).
 */
function estimateValueSize(value: Value): number {
  switch (value.type) {
    case ValueType.STRING:
      return value.value.length;
    case ValueType.ARRAY:
      return value.value.length;
    case ValueType.OBJECT:
      return value.value.length; // number of key-value pairs
    default:
      return 0;
  }
}

/**
 * Returns true if the value is a simple type that can be stringified synchronously.
 * Simple types: null, number, bool, function, capsule
 * Complex types that need async: object, array, string (strings can be very large)
 */
function isSimpleValue(value: Value): boolean {
  switch (value.type) {
    case ValueType.NULL:
    case ValueType.NUMBER:
    case ValueType.BOOL:
    case ValueType.FUNCTION:
    case ValueType.CAPSULE:
      return true;
    case ValueType.STRING:
    case ValueType.ARRAY:
    case ValueType.OBJECT:
      return false;
    default:
      return false;
  }
}

/**
 * Renders a stringified value.
 * Simple values (numbers, bools, etc.) are rendered synchronously.
 * Small complex values are also rendered synchronously.
 * Large complex values (that will exceed maxLength) are rendered asynchronously with a download link.
 */
const AsyncStringifiedValue = ({ value, maxLength = 50000 }: AsyncStringifiedValueProps) => {
  // Simple values can be rendered synchronously - they're fast and small
  if (isSimpleValue(value)) {
    return <>{alloyStringify(value)}</>;
  }

  // Check if this value is likely to exceed maxLength
  const isLargeValue = estimateValueSize(value) >= maxLength;

  if (isLargeValue) {
    // Large values use async rendering with download button
    return <AsyncLargeValue value={value} />;
  }

  // Small complex values can be rendered synchronously
  const result = alloyStringify(value);
  if (result.length > maxLength) {
    // Unexpectedly large after stringification - use download button
    return <DownloadButton result={result} />;
  }
  return <>{result}</>;
};

/**
 * Renders large values asynchronously with a download button.
 * Shows greyed-out download text immediately, becomes clickable when ready.
 */
const AsyncLargeValue = ({ value }: { value: Value }) => {
  const [state, setState] = useState<LargeValueState>({ status: 'loading' });

  useEffect(() => {
    let cancelled = false;

    // Use double-RAF to ensure the loading state is painted before blocking work starts
    requestAnimationFrame(() => {
      if (cancelled) return;
      requestAnimationFrame(() => {
        if (cancelled) return;

        try {
          const result = alloyStringify(value);
          if (!cancelled) {
            setState({ status: 'ready', result });
          }
        } catch {
          if (!cancelled) {
            setState({ status: 'ready', result: '[Error stringifying value]' });
          }
        }
      });
    });

    return () => {
      cancelled = true;
    };
  }, [value]);

  if (state.status === 'loading') {
    return (
      <span
        style={{
          display: 'inline-block',
          fontSize: '10px',
          padding: '5px',
          color: '#ffffff',
          backgroundColor: '#888',
          border: '1px solid #888',
          borderRadius: '3px',
        }}
      >
        <FontAwesomeIcon icon={faLink} /> Download the value contents
      </span>
    );
  }

  return <DownloadButton result={state.result} />;
};

/**
 * A button that downloads the given result as a file.
 */
const DownloadButton = ({ result }: { result: string }) => {
  const handleDownload = useCallback(() => {
    const blob = new Blob([result], { type: 'text/plain' });
    const url = URL.createObjectURL(blob);
    const link = document.createElement('a');
    link.href = url;
    link.download = 'value.json';
    document.body.appendChild(link);
    link.click();
    document.body.removeChild(link);
    URL.revokeObjectURL(url);
  }, [result]);

  return (
    <button
      onClick={handleDownload}
      style={{
        display: 'inline-block',
        fontSize: '10px',
        padding: '5px',
        color: '#ffffff',
        backgroundColor: 'rgb(56, 133, 220)',
        border: '1px solid rgb(56, 133, 220)',
        borderRadius: '3px',
        cursor: 'pointer',
      }}
    >
      <FontAwesomeIcon icon={faLink} /> Download the value contents
    </button>
  );
};

export default AsyncStringifiedValue;
