import React, { useEffect } from 'react';
import { useTranslation } from 'react-i18next';
import { useForm, Controller } from 'react-hook-form';
import ReactModal from 'react-modal';

import { Input } from '../../ui/Controls/Input';
import { Radio } from '../../ui/Controls/Radio';

export interface IpsetDefinition {
    name: string;
    type: string;
    family: string;
    timeout: number;
}

interface AutoCreateModalProps {
    isOpen: boolean;
    onClose: () => void;
    onSave: (definitions: IpsetDefinition[]) => void;
    initialDefinition?: IpsetDefinition | null;
    existingNames?: string[];
    title: string;
}

const R_IPSET_NAME = /^[A-Za-z0-9_-]+$/;

const AutoCreateModal: React.FC<AutoCreateModalProps> = ({
    isOpen,
    onClose,
    onSave,
    initialDefinition,
    existingNames = [],
    title,
}) => {
    const { t } = useTranslation();

    const { control, handleSubmit, reset } = useForm<IpsetDefinition>({
        defaultValues: {
            name: initialDefinition?.name || '',
            type: initialDefinition?.type || 'hash:ip',
            family: initialDefinition?.family || 'inet',
            timeout: initialDefinition?.timeout || 0,
        },
    });

    useEffect(() => {
        if (isOpen) {
            reset({
                name: initialDefinition?.name || '',
                type: initialDefinition?.type || 'hash:ip',
                family: initialDefinition?.family || 'inet',
                timeout: initialDefinition?.timeout || 0,
            });
        }
    }, [isOpen, initialDefinition, reset]);

    const onSubmit = (data: IpsetDefinition) => {
        const names = data.name
            .split(',')
            .map((n) => n.trim())
            .filter((n) => n.length > 0);

        const definitions = names.map((name) => ({
            name,
            type: data.type,
            family: data.family,
            timeout: data.timeout,
        }));

        onSave(definitions);
        reset();
        onClose();
    };

    const handleClose = () => {
        reset();
        onClose();
    };

    const validateName = (value: string) => {
        const names = (value || '')
            .split(',')
            .map((n) => n.trim())
            .filter((n) => n.length > 0);

        if (names.length === 0) {
            return t('ipset_autocreate_name_required');
        }

        const invalid = names.find((n) => !R_IPSET_NAME.test(n));
        if (invalid) {
            return `${t('ipset_error_name_chars')}: "${invalid}"`;
        }

        const duplicate = names.find((n) => existingNames.includes(n));
        if (duplicate) {
            return `${t('ipset_autocreate_name_duplicate')}: "${duplicate}"`;
        }

        return undefined;
    };

    const validateTimeout = (value: number) => {
        if (value < 0) {
            return t('ipset_error_timeout_negative');
        }
        return undefined;
    };

    const typeOptions = [
        { value: 'hash:ip', label: t('ipset_autocreate_type_ip') },
        { value: 'hash:net', label: t('ipset_autocreate_type_net') },
    ];

    const familyOptions = [
        { value: 'inet', label: t('ipset_autocreate_family_ipv4') },
        { value: 'inet6', label: t('ipset_autocreate_family_ipv6') },
    ];

    return (
        <ReactModal
            className="Modal__Bootstrap modal-dialog modal-dialog-centered"
            closeTimeoutMS={0}
            isOpen={isOpen}
            onRequestClose={handleClose}>
            <div className="modal-content">
                <div className="modal-header">
                    <h4 className="modal-title">{title}</h4>
                    <button type="button" className="close" onClick={handleClose}>
                        <span className="sr-only">Close</span>
                    </button>
                </div>
                <form onSubmit={handleSubmit(onSubmit)}>
                    <div className="modal-body">
                        <div className="form-group">
                            <Controller
                                name="name"
                                control={control}
                                rules={{ validate: validateName }}
                                render={({ field, fieldState }) => (
                                    <Input
                                        {...field}
                                        label={t('ipset_autocreate_name')}
                                        desc={t('ipset_autocreate_name_desc')}
                                        placeholder="my_ipset1, my_ipset2, my_ipset3"
                                        error={fieldState.error?.message}
                                    />
                                )}
                            />
                        </div>

                        <div className="form-group">
                            <label className="form__label">
                                {t('ipset_autocreate_type')}
                            </label>
                            <div className="form__desc mb-2">
                                {t('ipset_autocreate_type_desc')}
                            </div>
                            <Controller
                                name="type"
                                control={control}
                                render={({ field }) => (
                                    <Radio
                                        name="type"
                                        value={field.value}
                                        options={typeOptions}
                                        onChange={field.onChange}
                                    />
                                )}
                            />
                        </div>

                        <div className="form-group">
                            <label className="form__label">
                                {t('ipset_autocreate_family')}
                            </label>
                            <div className="form__desc mb-2">
                                {t('ipset_autocreate_family_desc')}
                            </div>
                            <Controller
                                name="family"
                                control={control}
                                render={({ field }) => (
                                    <Radio
                                        name="family"
                                        value={field.value}
                                        options={familyOptions}
                                        onChange={field.onChange}
                                    />
                                )}
                            />
                        </div>

                        <div className="form-group">
                            <Controller
                                name="timeout"
                                control={control}
                                rules={{ validate: validateTimeout }}
                                render={({ field, fieldState }) => (
                                    <Input
                                        {...field}
                                        type="number"
                                        min="0"
                                        step="1"
                                        label={t('ipset_autocreate_timeout')}
                                        desc={t('ipset_autocreate_timeout_desc')}
                                        placeholder="0"
                                        error={fieldState.error?.message}
                                        onChange={(e) => {
                                            const v = parseInt(e.target.value, 10);
                                            field.onChange(Number.isNaN(v) ? 0 : v);
                                        }}
                                    />
                                )}
                            />
                        </div>
                    </div>
                    <div className="modal-footer">
                        <button type="button" className="btn btn-secondary" onClick={handleClose}>
                            {t('cancel_btn')}
                        </button>
                        <button type="submit" className="btn btn-success">
                            {t('save_btn')}
                        </button>
                    </div>
                </form>
            </div>
        </ReactModal>
    );
};

export default AutoCreateModal;
