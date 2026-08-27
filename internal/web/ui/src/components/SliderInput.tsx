import { useId } from 'react';

import styles from './SliderInput.module.css';

interface SliderInputProps {
  label: string;
  min: number;
  max: number;
  value: number;
  defaultValue: number;
  valueLabel?: string;
  onChange: (value: number) => void;
  onCommit: (value: number) => void;
}

function normalizeValue(value: number, min: number, max: number, defaultValue: number) {
  if (Number.isNaN(value)) {
    return defaultValue;
  }
  return Math.min(Math.max(value, min), max);
}

function SliderInput({ label, min, max, value, defaultValue, valueLabel, onChange, onCommit }: Readonly<SliderInputProps>) {
  const inputId = useId();

  function handleChange(value: number) {
    onChange(normalizeValue(value, min, max, defaultValue));
  }

  function handleCommit(value: number) {
    const normalizedValue = normalizeValue(value, min, max, defaultValue);
    onChange(normalizedValue);
    onCommit(normalizedValue);
  }

  return (
    <div className={styles.slider}>
      <label className={styles.sliderLabel} htmlFor={inputId}>
        {label}
      </label>
      <input
        id={inputId}
        className={styles.rangeInput}
        type="range"
        min={min}
        max={max}
        value={value}
        onInput={(event) => handleChange(event.currentTarget.valueAsNumber)}
        onMouseUp={(event) => handleCommit(event.currentTarget.valueAsNumber)}
        onTouchEnd={(event) => handleCommit(event.currentTarget.valueAsNumber)}
        onKeyUp={(event) => handleCommit(event.currentTarget.valueAsNumber)}
        onBlur={(event) => handleCommit(event.currentTarget.valueAsNumber)}
      />
      <input
        className={styles.valueInput}
        type="number"
        min={min}
        max={max}
        value={value}
        aria-label={valueLabel ?? `${label} value`}
        onChange={(event) => handleChange(event.currentTarget.valueAsNumber)}
        onBlur={(event) => handleCommit(event.currentTarget.valueAsNumber)}
        onKeyDown={(event) => {
          if (event.key === 'Enter') {
            handleCommit(event.currentTarget.valueAsNumber);
          }
        }}
      />
    </div>
  );
}

export default SliderInput;
