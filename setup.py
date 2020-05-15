# -{pip install -e .}

from setuptools import setup

VERSION = '0.0.1'

setup(
    name='share',
    author="Kidus Adugna",
    author_email='kidusadugna@gmail.com',
    classifiers=[
        'Development Status :: 5 - Production/Stable',
        'Intended Audience :: Developers',
        'Intended Audience :: End Users/Desktop',
        'License :: OSI Approved :: MIT License',
        'Natural Language :: English',
        'Programming Language :: Python :: 3',
        'Programming Language :: Python :: 3.6',
        'Programming Language :: Python :: 3.7',
        'Programming Language :: Python :: 3.8',
    ],
    description="Share files in local networks",
    entry_points={
        'console_scripts': [
            'share=share.share:serve',
        ],
    },
    license="MIT license",
    # long_description=readme,
    long_description_content_type='text/markdown',
    include_package_data=True,
    keywords='lan, share',
    packages=['share'],
    # url='https://github.com/K1DV5/docal',
    version=VERSION,
    zip_safe=False,
)
