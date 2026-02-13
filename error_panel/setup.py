from setuptools import setup

setup(
    name='grafana-error-panel-cli',
    version='1.0.0',
    description='CLI tool to configure Grafana error panels via API',
    author='Your Name',
    py_modules=['grafana_error_panel_cli'],
    install_requires=[
        'click>=8.1.0',
        'requests>=2.31.0',
    ],
    entry_points={
        'console_scripts': [
            'grafana-error-panel=grafana_error_panel_cli:cli',
        ],
    },
    python_requires='>=3.7',
    classifiers=[
        'Development Status :: 4 - Beta',
        'Intended Audience :: Developers',
        'Topic :: System :: Monitoring',
        'Programming Language :: Python :: 3',
        'Programming Language :: Python :: 3.7',
        'Programming Language :: Python :: 3.8',
        'Programming Language :: Python :: 3.9',
        'Programming Language :: Python :: 3.10',
        'Programming Language :: Python :: 3.11',
    ],
)
