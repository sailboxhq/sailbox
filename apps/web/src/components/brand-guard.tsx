/*
 * SPDX-License-Identifier: AGPL-3.0-or-later
 * Copyright (c) 2026 Sailbox Authors
 *
 * AGPL v3 Section 7(b) attribution guard. DO NOT REMOVE.
 * See NOTICE in the repository root for full terms.
 */

import { useEffect, useRef } from "react";

// Cross-referenced in: sidebar.tsx, _dashboard.tsx
export const BRAND_ANCHOR = "sailbox-brand-attribution";
export const BRAND_TEXT = "Powered by Sailbox";
export const BRAND_URL = "https://github.com/sailboxhq/sailbox";
export const SPONSOR_URL = "https://github.com/sponsors/sailboxhq";

// Checks that the attribution DOM node stays present and visible.
// Logs a license warning if it's removed or hidden via CSS.
export function BrandGuard() {
  const warned = useRef(false);

  useEffect(() => {
    function check() {
      const el = document.getElementById(BRAND_ANCHOR);
      if (!el || el.offsetParent === null) {
        if (!warned.current) {
          console.warn(
            "⚠️ Sailbox attribution missing or hidden.\n" +
              'The AGPL v3 Section 7(b) terms require the "Powered by Sailbox"\n' +
              "notice to remain visible. See NOTICE for details.\n" +
              "Commercial licenses: https://github.com/sailboxhq/sailbox",
          );
          warned.current = true;
        }
      } else {
        warned.current = false;
      }
    }

    check();
    const id = setInterval(check, 30_000);
    return () => clearInterval(id);
  }, []);

  return null;
}
