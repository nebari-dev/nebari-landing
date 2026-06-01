import AxeBuilder from "@axe-core/playwright";
import { test as base, expect } from "./fixtures/e2e";

type AxeFixture = {
  makeAxeBuilder: () => AxeBuilder;
};

export const test = base.extend<AxeFixture>({
  makeAxeBuilder: async ({ page }, provide) => {
    const makeAxeBuilder = () =>
      new AxeBuilder({ page }).withTags(["wcag2a", "wcag2aa", "wcag21a", "wcag21aa"]);

    await provide(makeAxeBuilder);
  },
});

export { expect };
