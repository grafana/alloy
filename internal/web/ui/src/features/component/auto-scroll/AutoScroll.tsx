/**
 * @file Code adapted from https://github.com/brianmcallister/react-auto-scroll
 */

import React from 'react';
import classnames from 'classnames';

interface Props {
  /**
   * ID attribute of the checkbox.
   */
  checkBoxId?: string;
  /**
   * Children to render in the scroll container.
   */
  children: React.ReactNode;
  /**
   * Extra CSS class names.
   */
  className?: string;
  /**
   * Height value of the scroll container.
   */
  height?: number;
  /**
   * Text to use for the auto scroll option.
   */
  optionText?: string;
  /**
   * Prevent all mouse interaction with the scroll conatiner.
   */
  preventInteraction?: boolean;
  /**
   * Ability to disable the smooth scrolling behavior.
   */
  scrollBehavior?: 'smooth' | 'auto';
  /**
   * Current value if component should auto scroll.
   */
  autoScroll: boolean;
  /**
   * Setter for {@link autoScroll} that will be called internally.
   */
  setAutoScroll: React.Dispatch<React.SetStateAction<boolean>>;
}

/**
 * Base CSS class.
 * @private
 */
const baseClass = 'react-auto-scroll';

/**
 * Get a random string.
 * @private
 */
const getRandomString = () => Math.random().toString(36).slice(2, 15);

/**
 * AutoScroll component.
 */
export default function AutoScroll({
  checkBoxId = getRandomString(),
  children,
  className,
  height,
  optionText = 'Auto scroll',
  preventInteraction = false,
  scrollBehavior = 'smooth',
  autoScroll,
  setAutoScroll,
}: Props) {
  const containerElement = React.useRef<HTMLDivElement>(null);
  const cls = classnames(baseClass, className, {
    [`${baseClass}--empty`]: React.Children.count(children) === 0,
    [`${baseClass}--prevent-interaction`]: preventInteraction,
  });
  const style = {
    height,
    overflow: 'auto',
    scrollBehavior: 'auto',
    pointerEvents: preventInteraction ? 'none' : 'auto',
  } as const;

  // Handle mousewheel events on the scroll container.
  const onWheel = () => {
    const { current } = containerElement;

    if (current) {
      setAutoScroll(current.scrollTop + current.offsetHeight === current.scrollHeight);
    }
  };

  // Apply the scroll behavior property after the first render,
  // so that the initial render is scrolled all the way to the bottom.
  React.useEffect(() => {
    setTimeout(() => {
      const { current } = containerElement;

      if (current) {
        current.style.scrollBehavior = scrollBehavior;
      }
    }, 0);
  }, [containerElement, scrollBehavior]);

  // When the children are updated, scroll the container
  // to the bottom.
  React.useEffect(() => {
    if (!autoScroll) {
      return;
    }

    const { current } = containerElement;

    if (current) {
      current.scrollTop = current.scrollHeight;
    }
  }, [children, containerElement, autoScroll]);

  return (
    <div className={cls}>
      <div className={`${baseClass}__scroll-container`} onWheel={onWheel} ref={containerElement} style={style}>
        {children}
      </div>
    </div>
  );
}
