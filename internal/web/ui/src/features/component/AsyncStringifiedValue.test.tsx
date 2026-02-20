import { act, render, screen } from '@testing-library/react';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import { type Value, ValueType } from '../alloy-syntax-js/types';

import AsyncStringifiedValue from './AsyncStringifiedValue';

// Import the large fixture data generator
// This generates a structure similar to real discovery.kubernetes output
import { largeDiscOutput } from '../../test/fixtures/generateLargeDiscOutput';

describe('AsyncStringifiedValue', () => {
  describe('simple values render synchronously', () => {
    it('renders number values immediately', () => {
      const numberValue: Value = { type: ValueType.NUMBER, value: 42 };

      render(<AsyncStringifiedValue value={numberValue} />);

      expect(screen.getByText('42')).toBeInTheDocument();
      expect(screen.queryByText('Loading...')).not.toBeInTheDocument();
    });

    it('renders boolean values immediately', () => {
      const boolValue: Value = { type: ValueType.BOOL, value: true };

      render(<AsyncStringifiedValue value={boolValue} />);

      expect(screen.getByText('true')).toBeInTheDocument();
      expect(screen.queryByText('Loading...')).not.toBeInTheDocument();
    });

    it('renders null values immediately', () => {
      const nullValue: Value = { type: ValueType.NULL };

      render(<AsyncStringifiedValue value={nullValue} />);

      expect(screen.getByText('null')).toBeInTheDocument();
      expect(screen.queryByText('Loading...')).not.toBeInTheDocument();
    });
  });

  describe('complex values render asynchronously with size check', () => {
    beforeEach(() => {
      vi.useFakeTimers();
    });

    afterEach(() => {
      vi.useRealTimers();
    });

    it('shows nothing initially for strings (delayed loading indicator)', () => {
      const stringValue: Value = { type: ValueType.STRING, value: 'hello world' };

      const { container } = render(<AsyncStringifiedValue value={stringValue} />);

      // During the delay threshold, nothing is shown (avoids flickering for fast ops)
      expect(screen.queryByText('Loading...')).not.toBeInTheDocument();
      expect(container.textContent).toBe('');
    });

    it('skips loading state entirely when processing completes before delay threshold', async () => {
      const stringValue: Value = { type: ValueType.STRING, value: 'hello world' };

      render(<AsyncStringifiedValue value={stringValue} />);

      // Run all timers - for fast operations, loading state is never shown
      await act(async () => {
        vi.runAllTimers();
      });

      // The result should appear without ever showing "Loading..."
      expect(screen.getByText('"hello world"')).toBeInTheDocument();
      // Verify loading was never shown (it was skipped because processing was fast)
      expect(screen.queryByText('Loading...')).not.toBeInTheDocument();
    });

    it('renders small strings after processing', async () => {
      const stringValue: Value = { type: ValueType.STRING, value: 'hello world' };

      render(<AsyncStringifiedValue value={stringValue} />);

      await act(async () => {
        vi.runAllTimers();
      });

      expect(screen.getByText('"hello world"')).toBeInTheDocument();
    });

    it('shows download link for large strings', async () => {
      const largeString = 'x'.repeat(3000);
      const stringValue: Value = { type: ValueType.STRING, value: largeString };

      render(<AsyncStringifiedValue value={stringValue} maxLength={100} />);

      await act(async () => {
        vi.runAllTimers();
      });

      const downloadButton = screen.getByRole('button', { name: 'Download the value contents' });
      expect(downloadButton).toBeInTheDocument();
    });

    it('shows nothing initially for arrays (delayed loading indicator)', () => {
      const arrayValue: Value = {
        type: ValueType.ARRAY,
        value: [{ type: ValueType.NUMBER, value: 1 }],
      };

      const { container } = render(<AsyncStringifiedValue value={arrayValue} />);

      // During the delay threshold, nothing is shown (avoids flickering for fast ops)
      expect(screen.queryByText('Loading...')).not.toBeInTheDocument();
      expect(container.textContent).toBe('');
    });

    it('renders small arrays after processing', async () => {
      const arrayValue: Value = {
        type: ValueType.ARRAY,
        value: [
          { type: ValueType.NUMBER, value: 1 },
          { type: ValueType.NUMBER, value: 2 },
        ],
      };

      render(<AsyncStringifiedValue value={arrayValue} />);

      // Run all timers and frames
      await act(async () => {
        vi.runAllTimers();
      });

      // Small array should render successfully
      expect(screen.getByText('[1, 2]')).toBeInTheDocument();
    });

    it('shows download link for values exceeding maxLength', async () => {
      // The large value is in exports[0].value (the targets array)
      const largeValue = largeDiscOutput.exports[0].value as Value;

      // Use a small maxLength to ensure it exceeds
      render(<AsyncStringifiedValue value={largeValue} maxLength={100} />);

      await act(async () => {
        vi.runAllTimers();
      });

      // Should show download link instead of error message
      const downloadButton = screen.getByRole('button', { name: 'Download the value contents' });
      expect(downloadButton).toBeInTheDocument();
    });

    it('shows download link with default 50000 char limit', async () => {
      const largeValue = largeDiscOutput.exports[0].value as Value;

      render(<AsyncStringifiedValue value={largeValue} />);

      await act(async () => {
        vi.runAllTimers();
      });

      // Should show download link (fixture is much larger than 50000 chars)
      const downloadButton = screen.getByRole('button', { name: 'Download the value contents' });
      expect(downloadButton).toBeInTheDocument();
    });

    it('cleans up properly when unmounted during processing', async () => {
      const largeValue = largeDiscOutput.exports[0].value as Value;

      const { unmount } = render(<AsyncStringifiedValue value={largeValue} />);

      // Unmount while still processing (before loading or result appears)
      unmount();

      // Run all timers - should not throw any errors
      await act(async () => {
        vi.runAllTimers();
      });

      expect(true).toBe(true);
    });
  });
});
