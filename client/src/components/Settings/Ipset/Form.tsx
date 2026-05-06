import React, { useEffect, useState } from 'react';
import { Controller, useForm } from 'react-hook-form';
import { useTranslation } from 'react-i18next';
import { useDispatch } from 'react-redux';

import RulesTable from './RulesTable';
import RuleModal from './RuleModal';
import AutoCreateTable from './AutoCreateTable';
import AutoCreateModal, { IpsetDefinition } from './AutoCreateModal';
import { parseIPSetRule, isDuplicateRule, validateIPSetRule } from '../../../helpers/ipset';
import { addErrorToast } from '../../../actions/toasts';
import { Radio } from '../../ui/Controls/Radio';
import { Input } from '../../ui/Controls/Input';
import { Checkbox } from '../../ui/Controls/Checkbox';

interface IpsetCreateConfig {
    enabled: boolean;
    sets: IpsetDefinition[];
}

interface FormProps {
    initialRules: string[];
    initialFilePath: string;
    initialIpsetCreate: IpsetCreateConfig | null;
    onSubmit: (data: { ipset: string[]; ipset_file: string; ipset_create: IpsetCreateConfig | null }) => void;
    processing: boolean;
}

type StorageMode = 'config' | 'file';

interface FormData {
    mode: StorageMode;
    rules: string[];
    filePath: string;
    autoCreateEnabled: boolean;
    autoCreateSets: IpsetDefinition[];
}

const buildDefaults = (
    rules: string[],
    filePath: string,
    ipsetCreate: IpsetCreateConfig | null,
): FormData => ({
    mode: filePath && filePath.trim() !== '' ? 'file' : 'config',
    rules,
    filePath,
    autoCreateEnabled: ipsetCreate?.enabled || false,
    autoCreateSets: ipsetCreate?.sets || [],
});

const Form: React.FC<FormProps> = ({
    initialRules,
    initialFilePath,
    initialIpsetCreate,
    onSubmit,
    processing,
}) => {
    const { t } = useTranslation();
    const dispatch = useDispatch();

    const {
        control,
        handleSubmit,
        watch,
        setValue,
        reset,
        formState: { isDirty, isSubmitting },
    } = useForm<FormData>({
        defaultValues: buildDefaults(initialRules, initialFilePath, initialIpsetCreate),
    });

    useEffect(() => {
        reset(buildDefaults(initialRules, initialFilePath, initialIpsetCreate));
    }, [initialRules, initialFilePath, initialIpsetCreate, reset]);

    const mode = watch('mode');
    const rules = watch('rules');
    const autoCreateEnabled = watch('autoCreateEnabled');
    const autoCreateSets = watch('autoCreateSets');

    const [isModalOpen, setIsModalOpen] = useState(false);
    const [editingIndex, setEditingIndex] = useState<number | null>(null);
    const [isAutoCreateModalOpen, setIsAutoCreateModalOpen] = useState(false);
    const [editingAutoCreateIndex, setEditingAutoCreateIndex] = useState<number | null>(null);

    const setRules = (next: string[]) => {
        setValue('rules', next, { shouldDirty: true });
    };

    const setAutoCreateSets = (next: IpsetDefinition[]) => {
        setValue('autoCreateSets', next, { shouldDirty: true });
    };

    const handleAddRule = () => {
        setEditingIndex(null);
        setIsModalOpen(true);
    };

    const handleEditRule = (index: number) => {
        setEditingIndex(index);
        setIsModalOpen(true);
    };

    const handleSaveRule = (newRule: string) => {
        const error = validateIPSetRule(newRule);
        if (error) {
            dispatch(addErrorToast({ error: new Error(`${t('ipset_invalid_rule_prefix')}: ${error}`) }));
            return;
        }

        const otherRules = editingIndex !== null
            ? rules.filter((_, i) => i !== editingIndex)
            : rules;

        if (isDuplicateRule(newRule, otherRules)) {
            dispatch(addErrorToast({ error: new Error(t('ipset_duplicate_rule')) }));
            return;
        }

        if (editingIndex !== null) {
            const next = [...rules];
            next[editingIndex] = newRule;
            setRules(next);
        } else {
            setRules([...rules, newRule]);
        }
    };

    const handleDeleteRule = (index: number) => {
        if (window.confirm(t('ipset_confirm_delete'))) {
            setRules(rules.filter((_, i) => i !== index));
        }
    };

    const handleAddAutoCreateSet = () => {
        setEditingAutoCreateIndex(null);
        setIsAutoCreateModalOpen(true);
    };

    const handleEditAutoCreateSet = (index: number) => {
        setEditingAutoCreateIndex(index);
        setIsAutoCreateModalOpen(true);
    };

    const handleSaveAutoCreateSet = (definitions: IpsetDefinition[]) => {
        if (editingAutoCreateIndex !== null) {
            const next = [...autoCreateSets];
            [next[editingAutoCreateIndex]] = definitions;
            setAutoCreateSets(next);
        } else {
            setAutoCreateSets([...autoCreateSets, ...definitions]);
        }
    };

    const handleDeleteAutoCreateSet = (index: number) => {
        if (window.confirm(t('ipset_autocreate_confirm_delete'))) {
            setAutoCreateSets(autoCreateSets.filter((_, i) => i !== index));
        }
    };

    const onFormSubmit = (data: FormData) => {
        const ipsetCreate: IpsetCreateConfig = {
            enabled: data.autoCreateEnabled,
            sets: data.autoCreateSets,
        };

        if (data.mode === 'file') {
            if (!data.filePath || data.filePath.trim() === '') {
                dispatch(addErrorToast({ error: new Error(t('ipset_file_path_required')) }));
                return;
            }
            onSubmit({ ipset: [], ipset_file: data.filePath.trim(), ipset_create: ipsetCreate });
            return;
        }

        const invalidRule = data.rules.find((rule) => validateIPSetRule(rule) !== undefined);
        if (invalidRule) {
            const err = validateIPSetRule(invalidRule);
            dispatch(addErrorToast({
                error: new Error(`${t('ipset_invalid_rule_prefix')} "${invalidRule}": ${err}`),
            }));
            return;
        }
        onSubmit({ ipset: data.rules, ipset_file: '', ipset_create: ipsetCreate });
    };

    const modeOptions = [
        { value: 'config', label: t('ipset_mode_config') },
        { value: 'file', label: t('ipset_mode_file') },
    ];

    const editingRule = editingIndex !== null ? rules[editingIndex] : null;
    const parsedEditingRule = editingRule ? parseIPSetRule(editingRule) : null;

    const existingAutoCreateNames = autoCreateSets
        .map((s, i) => (i === editingAutoCreateIndex ? null : s.name))
        .filter((n): n is string => n !== null);

    return (
        <form onSubmit={handleSubmit(onFormSubmit)}>
            <div className="row">
                <div className="col-12">
                    <div className="form__group form__group--settings">
                        <label className="form__label form__label--with-desc">
                            {t('ipset_storage_mode')}
                        </label>
                        <div className="form__desc form__desc--top">{t('ipset_storage_mode_desc')}</div>
                        <div className="custom-controls-stacked">
                            <Controller
                                name="mode"
                                control={control}
                                render={({ field }) => (
                                    <Radio
                                        name="storage_mode"
                                        value={field.value}
                                        options={modeOptions}
                                        disabled={processing}
                                        onChange={(value) => field.onChange(value as StorageMode)}
                                    />
                                )}
                            />
                        </div>
                    </div>
                </div>

                {mode === 'file' ? (
                    <div className="col-12 col-md-7">
                        <div className="form__group form__group--settings">
                            <Controller
                                name="filePath"
                                control={control}
                                render={({ field }) => (
                                    <Input
                                        {...field}
                                        label={t('ipset_file_path')}
                                        desc={t('ipset_file_path_desc')}
                                        placeholder="/etc/adguardhome/ipset.conf"
                                        disabled={processing}
                                    />
                                )}
                            />
                        </div>
                    </div>
                ) : (
                    <div className="col-12">
                        <div className="form__group form__group--settings">
                            <label className="form__label">{t('ipset_rules')}</label>
                            <div className="form__desc mb-3">{t('ipset_rules_desc')}</div>

                            <button
                                type="button"
                                className="btn btn-success btn-sm mb-3"
                                onClick={handleAddRule}
                                disabled={processing}>
                                + {t('ipset_add_rule')}
                            </button>

                            <RulesTable
                                rules={rules}
                                onEdit={handleEditRule}
                                onDelete={handleDeleteRule}
                                disabled={processing}
                            />
                        </div>
                    </div>
                )}

                <div className="col-12">
                    <hr className="my-4" />
                    <div className="form__group form__group--settings">
                        <label className="form__label form__label--with-desc">
                            {t('ipset_autocreate_title')}
                        </label>
                        <div className="form__desc form__desc--top mb-3">
                            {t('ipset_autocreate_desc')}
                        </div>
                        <Controller
                            name="autoCreateEnabled"
                            control={control}
                            render={({ field }) => (
                                <Checkbox
                                    name="autocreate_enabled"
                                    value={field.value}
                                    title={t('ipset_autocreate_enable')}
                                    disabled={processing}
                                    onChange={() => field.onChange(!field.value)}
                                />
                            )}
                        />
                    </div>

                    {autoCreateEnabled && (
                        <div className="form__group form__group--settings mt-4">
                            <label className="form__label">{t('ipset_autocreate_sets')}</label>
                            <div className="form__desc mb-3">{t('ipset_autocreate_sets_desc')}</div>

                            <button
                                type="button"
                                className="btn btn-success btn-sm mb-3"
                                onClick={handleAddAutoCreateSet}
                                disabled={processing}>
                                + {t('ipset_autocreate_add')}
                            </button>

                            <AutoCreateTable
                                definitions={autoCreateSets}
                                onEdit={handleEditAutoCreateSet}
                                onDelete={handleDeleteAutoCreateSet}
                                disabled={processing}
                            />
                        </div>
                    )}
                </div>

                <div className="col-12">
                    <div className="alert alert-info">
                        <strong>{t('ipset_info_title')}:</strong>
                        <p className="mb-1">{t('ipset_info_desc')}</p>
                        <ul className="mb-0">
                            <li>{t('ipset_info_linux_only')}</li>
                            <li>{t('ipset_info_format')}</li>
                        </ul>
                    </div>
                </div>
            </div>

            <button
                type="submit"
                className="btn btn-success btn-standard btn-large"
                disabled={!isDirty || isSubmitting || processing}>
                {t('save_btn')}
            </button>

            <AutoCreateModal
                isOpen={isAutoCreateModalOpen}
                onClose={() => setIsAutoCreateModalOpen(false)}
                onSave={handleSaveAutoCreateSet}
                initialDefinition={editingAutoCreateIndex !== null ? autoCreateSets[editingAutoCreateIndex] : null}
                existingNames={existingAutoCreateNames}
                title={editingAutoCreateIndex !== null ? t('ipset_autocreate_edit') : t('ipset_autocreate_add')}
            />

            <RuleModal
                isOpen={isModalOpen}
                onClose={() => setIsModalOpen(false)}
                onSave={handleSaveRule}
                initialDomains={parsedEditingRule?.domains.join(',') || ''}
                initialIPSets={parsedEditingRule?.ipsets.join(',') || ''}
                title={editingIndex !== null ? t('ipset_edit_rule') : t('ipset_add_rule')}
            />
        </form>
    );
};

export default Form;
