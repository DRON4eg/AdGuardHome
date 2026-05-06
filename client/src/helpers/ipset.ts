/**
 * IPSet utilities for parsing and validating ipset rules
 * Rule format: DOMAIN[,DOMAIN,...]/IPSET_NAME[,IPSET_NAME,...]
 */

import i18next from 'i18next';

import { validateDomain } from './validators';

export interface IPSetRule {
    domains: string[];
    ipsets: string[];
}

const R_IPSET_NAME = /^[a-zA-Z0-9_-]+$/;

export const parseIPSetRule = (rule: string): IPSetRule | null => {
    const trimmedRule = rule.trim();
    if (!trimmedRule) {
        return null;
    }

    const separatorIndex = trimmedRule.indexOf('/');
    if (separatorIndex === -1) {
        return null;
    }

    const domains = trimmedRule
        .substring(0, separatorIndex)
        .split(',')
        .map((d) => d.trim())
        .filter((d) => d.length > 0);

    const ipsets = trimmedRule
        .substring(separatorIndex + 1)
        .split(',')
        .map((s) => s.trim())
        .filter((s) => s.length > 0);

    if (domains.length === 0 || ipsets.length === 0) {
        return null;
    }

    return { domains, ipsets };
};

export const formatIPSetRule = (rule: IPSetRule): string => {
    return `${rule.domains.join(',')}/${rule.ipsets.join(',')}`;
};

export const validateIPSetName = (name: string): string | undefined => {
    const trimmed = name.trim();
    if (trimmed.length === 0) {
        return i18next.t('ipset_error_name_empty');
    }
    if (!R_IPSET_NAME.test(trimmed)) {
        return i18next.t('ipset_error_name_chars');
    }
    return undefined;
};

export const validateIPSetRule = (rule: string): string | undefined => {
    const trimmedRule = rule.trim();
    if (!trimmedRule) {
        return i18next.t('ipset_error_rule_empty');
    }

    const parsed = parseIPSetRule(trimmedRule);
    if (!parsed) {
        return i18next.t('ipset_error_rule_format');
    }

    const invalidDomain = parsed.domains.find((d) => validateDomain(d) !== undefined);
    if (invalidDomain) {
        return `${invalidDomain}: ${validateDomain(invalidDomain)}`;
    }

    const invalidIpset = parsed.ipsets.find((s) => validateIPSetName(s) !== undefined);
    if (invalidIpset) {
        return `${invalidIpset}: ${validateIPSetName(invalidIpset)}`;
    }

    return undefined;
};

export const validateDomainsInput = (input: string): string | undefined => {
    const domains = input
        .split(',')
        .map((d) => d.trim())
        .filter((d) => d.length > 0);

    if (domains.length === 0) {
        return i18next.t('ipset_error_domains_required');
    }

    const invalidDomain = domains.find((d) => validateDomain(d) !== undefined);
    if (invalidDomain) {
        return `${invalidDomain}: ${validateDomain(invalidDomain)}`;
    }

    return undefined;
};

export const validateIPSetsInput = (input: string): string | undefined => {
    const ipsets = input
        .split(',')
        .map((s) => s.trim())
        .filter((s) => s.length > 0);

    if (ipsets.length === 0) {
        return i18next.t('ipset_error_ipsets_required');
    }

    const invalidIpset = ipsets.find((s) => validateIPSetName(s) !== undefined);
    if (invalidIpset) {
        return `${invalidIpset}: ${validateIPSetName(invalidIpset)}`;
    }

    return undefined;
};

export const isDuplicateRule = (rule: string, existingRules: string[]): boolean => {
    const trimmedRule = rule.trim();
    return existingRules.some((r) => r.trim() === trimmedRule);
};
