/**
 * Custom versioning strategy that bumps minor version for breaking changes
 * instead of major version. Extends the default strategy and swaps
 * MajorVersionUpdate for MinorVersionUpdate.
 */

import { DefaultVersioningStrategy } from 'release-please/build/src/versioning-strategies/default.js';
import { MajorVersionUpdate, MinorVersionUpdate } from 'release-please/build/src/versioning-strategy.js';

export class MinorBreakingVersioningStrategy extends DefaultVersioningStrategy {
  /**
   * Override to return MinorVersionUpdate instead of MajorVersionUpdate
   * when there are breaking changes.
   */
  determineReleaseType(version, commits) {
    const releaseType = super.determineReleaseType(version, commits);

    // If the default strategy would do a major bump, do a minor bump instead
    if (
      releaseType instanceof MajorVersionUpdate ||
      releaseType.constructor.name === 'MajorVersionUpdate'
    ) {
      console.log('Breaking changes detected - bumping minor version instead of major');
      return new MinorVersionUpdate();
    }

    return releaseType;
  }
}
